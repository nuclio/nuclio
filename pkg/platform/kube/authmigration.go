/*
Copyright 2026 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"golang.org/x/sync/semaphore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const authMigrationConcurrency = 5

// functionAuthMigration is what step 1 decided for a single function.
type functionAuthMigration struct {
	function *nuclioio.NuclioFunction

	// empty when there is nothing to write and the function only needs the migration label - it already
	// declares a mode, it has no HTTP trigger to hold one, or it is mid-deploy
	mode auth.AuthenticationMode

	// set only for basicAuth: the credentials live in that gateway's own secret and must be re-scrubbed
	// into the function's secret, because a $ref resolves only against the secret of its owner. When
	// several basicAuth gateways front the function, this holds the newest one
	basicAuthAPIGateway *nuclioio.NuclioAPIGateway
	migrated            bool
}

// MigrateFunctionAuthentication moves authentication off the api gateways and onto each function's HTTP
// trigger. Without it, a function deployed before the feature flag was turned on carries no authentication
// mode and gets no auth-proxy sidecar, while its api gateway ingress is already rendered without auth
// annotations - leaving the function open. Runs in the background on dashboard startup, and is idempotent
// because every migrated CRD is labeled and never listed again.
//
// The migration has three steps:
//  1. list the unmigrated resources and decide which authentication mode each function should get
//  2. write that mode onto the functions
//  3. drain the authentication off the api gateways whose functions all migrated
func (p *Platform) MigrateFunctionAuthentication(ctx context.Context) {
	if !p.IsFunctionAuthenticationEnabled() {
		p.Logger.InfoWithCtx(ctx, "Function authentication migration skipped: feature flag is off")
		return
	}

	namespace := common.ResolveDefaultNamespace(p.DefaultNamespace)
	p.Logger.InfoWithCtx(ctx, "Starting function authentication migration", "namespace", namespace)

	p.Logger.InfoWithCtx(ctx, "Resolving the authentication mode of each function (step 1/3)")
	functions, apiGateways, err := p.listUnmigratedResources(ctx, namespace)
	if err != nil {
		// a failed list should not block the dashboard from starting, and the whole migration is
		// retried on the next restart
		p.Logger.WarnWithCtx(ctx, "Failed to list resources pending authentication migration; skipping",
			"err", err.Error())
		return
	}

	functionMigrations := p.resolveFunctionAuthMigrations(ctx, functions, apiGateways)

	// functions first: a gateway keeps its authentication until every function behind it carries the mode
	// translated from it
	p.Logger.InfoWithCtx(ctx, "Migrating functions (step 2/3)")
	p.migrateFunctions(ctx, namespace, functionMigrations)

	p.Logger.InfoWithCtx(ctx, "Migrating api gateways (step 3/3)")
	p.migrateAPIGateways(ctx, namespace, apiGateways, functionMigrations)

	p.Logger.InfoWithCtx(ctx, "Finished function authentication migration")
}

// listUnmigratedResources lists only the CRDs that carry no migration label yet, so once the migration has
// run, every later restart lists nothing.
func (p *Platform) listUnmigratedResources(ctx context.Context,
	namespace string) ([]nuclioio.NuclioFunction, []nuclioio.NuclioAPIGateway, error) {

	unmigratedSelector := fmt.Sprintf("!%s", common.NuclioLabelKeyMigrationFunctionAuth)

	functionList, err := p.consumer.NuclioClientSet.ListNuclioFunctions(ctx,
		namespace,
		metav1.ListOptions{LabelSelector: unmigratedSelector})
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to list unmigrated functions")
	}

	apiGatewayList, err := p.consumer.NuclioClientSet.ListNuclioAPIGateways(ctx,
		namespace,
		metav1.ListOptions{LabelSelector: unmigratedSelector})
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to list unmigrated api gateways")
	}

	return functionList.Items, apiGatewayList.Items, nil
}

// resolveFunctionAuthMigrations is step 1: it decides which authentication mode each function should get,
// translated from the api gateways in front of it.
func (p *Platform) resolveFunctionAuthMigrations(ctx context.Context,
	functions []nuclioio.NuclioFunction,
	apiGateways []nuclioio.NuclioAPIGateway) map[string]*functionAuthMigration {

	migrations := make(map[string]*functionAuthMigration, len(functions))

	// the subset of the above still waiting for a mode, for the api gateway pass below
	migrationsWaitingForMode := make(map[string]*functionAuthMigration, len(functions))

	for _, function := range functions {
		migration := &functionAuthMigration{function: &function}
		migrations[function.Name] = migration

		// a stale function is mid-deploy, so its spec is not ours to write - mark it and leave the spec
		// alone. It cannot come up unauthenticated: the stale-function sweep fails it, and the redeploy the
		// user then has to run enriches the authentication mode on its own
		if functionconfig.FunctionStateStale(function.Status.State) {
			p.Logger.DebugWithCtx(ctx, "Function is stale, marking as migrated without setting a mode",
				"functionName", function.Name,
				"functionState", function.Status.State)
			continue
		}

		currentMode, err := functionconfig.GetHTTPTriggerMode(function.Spec.Triggers)
		if err != nil {
			// no single HTTP trigger means there is nowhere to put a mode, so the function is migrated by
			// labeling it alone
			p.Logger.DebugWithCtx(ctx, "Function has no single HTTP trigger, marking as migrated",
				"functionName", function.Name,
				"err", err.Error())
			continue
		}
		if currentMode != "" {
			// already on the new model, whether the user set it or the deploy-time enrichment did
			p.Logger.DebugWithCtx(ctx, "Function already declares an authentication mode, marking as migrated",
				"functionName", function.Name,
				"authenticationMode", currentMode)
			continue
		}

		migrationsWaitingForMode[function.Name] = migration
	}

	// a gateway's mode is offered to every function behind it, so a canary gateway covers both of its
	// targets, and a function behind gateways that disagree keeps the highest ranked mode
	for _, apiGateway := range apiGateways {
		gatewayAuthenticationMode, err := translateAPIGatewayAuthenticationMode(&apiGateway)
		if err != nil {
			// an unmappable mode must never leave a function unauthenticated
			p.Logger.ErrorWithCtx(ctx,
				"Failed to translate api gateway authentication mode, skipping the gateway authentication resolving",
				"apiGatewayName", apiGateway.Name,
				"gatewayAuthenticationMode", apiGateway.Spec.AuthenticationMode,
				"err", err.Error())
			continue
		}

		for _, upstream := range apiGateway.Spec.Upstreams {
			if upstream.NuclioFunction == nil {
				continue
			}

			migration, waitingForMode := migrationsWaitingForMode[upstream.NuclioFunction.Name]
			if !waitingForMode {
				continue
			}

			chosenMode := p.chooseAuthByPriority(migration.mode, gatewayAuthenticationMode)
			if migration.mode != "" && migration.mode != gatewayAuthenticationMode {
				p.Logger.DebugWithCtx(ctx,
					"Function is fronted by api gateways with divergent authentication, taking the higher priority",
					"functionName", migration.function.Name,
					"apiGatewayName", apiGateway.Name,
					"previousMode", migration.mode,
					"gatewayAuthenticationMode", gatewayAuthenticationMode,
					"chosenMode", chosenMode)
			}
			migration.mode = chosenMode

			if gatewayAuthenticationMode == auth.AuthenticationModeBasicAuth {
				if migration.basicAuthAPIGateway == nil {
					migration.basicAuthAPIGateway = &apiGateway
				} else {
					// the function's HTTP trigger holds a single username and password, so only one
					// gateway's credentials can move onto it
					chosen, dropped := migration.basicAuthAPIGateway, &apiGateway
					if isPreferredBasicAuthAPIGateway(dropped, chosen) {
						chosen, dropped = dropped, chosen
					}
					p.Logger.WarnWithCtx(ctx,
						"Function is fronted by multiple basicAuth api gateways, keeping the newest one's credentials",
						"functionName", migration.function.Name,
						"chosenAPIGatewayName", chosen.Name,
						"droppedAPIGatewayName", dropped.Name)
					migration.basicAuthAPIGateway = chosen
				}
			}
		}
	}

	for _, migration := range migrationsWaitingForMode {
		if migration.mode == "" {
			// no gateway carried authentication, so the function keeps no authentication
			p.Logger.DebugWithCtx(ctx,
				"No api gateway with authentication fronted the function, leaving it unauthenticated",
				"functionName", migration.function.Name)
		}

		p.Logger.InfoWithCtx(ctx, "Resolved function authentication mode",
			"functionName", migration.function.Name,
			"authenticationMode", migration.mode)
	}

	return migrations
}

// authenticationModeRank ranks the modes for the tie-break between gateways that disagree. Higher wins: the
// platform default first, then the other credential-less mode, and basicAuth last because it is the only
// mode carrying per-function credentials.
func (p *Platform) authenticationModeRank(mode auth.AuthenticationMode) int {
	switch mode {
	// lowest, so that authentication on any one gateway beats none - even when none is the platform default
	case "", auth.AuthenticationModeNone:
		return 0
	// highest, so that the platform default beats any other mode by design
	case p.Config.Authentication.DefaultMode:
		return 4
	case auth.AuthenticationModeAPI:
		return 3
	case auth.AuthenticationModeBrowser:
		return 2
	case auth.AuthenticationModeBasicAuth:
		return 1
	default:
		return 0
	}
}

// chooseAuthByPriority returns whichever of the two modes outranks the other, keeping the first on a tie.
func (p *Platform) chooseAuthByPriority(modeA, modeB auth.AuthenticationMode) auth.AuthenticationMode {
	if p.authenticationModeRank(modeB) > p.authenticationModeRank(modeA) {
		return modeB
	}
	return modeA
}

// isPreferredBasicAuthAPIGateway reports whether candidate should replace chosen as the credential source of
// a function fronted by several basicAuth gateways: the newest gateway wins, and because creationTimestamp
// is only second-granular, the greater name breaks a tie. Both keys are immutable, so the answer is the same
// on every run and never depends on the order the gateways were listed in.
func isPreferredBasicAuthAPIGateway(candidate, chosen *nuclioio.NuclioAPIGateway) bool {
	if candidate.CreationTimestamp.Equal(&chosen.CreationTimestamp) {
		return candidate.Name > chosen.Name
	}
	return chosen.CreationTimestamp.Before(&candidate.CreationTimestamp)
}

// migrateFunctions is step 2: it writes the resolved mode onto each function. A failure is not fatal - the
// function stays unlabeled and is retried on the next dashboard restart, and step 3 leaves the authentication
// on the api gateways in front of it.
func (p *Platform) migrateFunctions(ctx context.Context,
	namespace string,
	functionsToMigrate map[string]*functionAuthMigration) {
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(authMigrationConcurrency)

	for _, function := range functionsToMigrate {
		if err := sem.Acquire(ctx, 1); err != nil {
			p.Logger.WarnWithCtx(ctx, "Failed to acquire a migration slot, stopping function migration",
				"err", err.Error())
			break
		}

		wg.Go(func() {
			defer sem.Release(1)
			if err := p.migrateFunction(ctx, namespace, function); err != nil {
				p.Logger.WarnWithCtx(ctx, "Failed to migrate function authentication",
					"functionName", function.function.Name,
					"err", err.Error())
				return
			}
			p.Logger.DebugWithCtx(ctx, "Successfully migrated function", "functionName", function.function.Name)
			function.migrated = true
		})
	}
	wg.Wait()
}

func (p *Platform) migrateFunction(ctx context.Context,
	namespace string,
	migration *functionAuthMigration) error {
	function := migration.function
	p.Logger.InfoWithCtx(ctx, "Migrating function authentication",
		"functionName", function.Name,
		"authentication mode", migration.mode)

	if migration.mode != "" {
		if err := p.setFunctionAuthentication(ctx, function, migration); err != nil {
			return errors.Wrap(err, "Failed to set the function's authentication mode")
		}
	}

	markMigratedToFunctionAuthentication(function)

	if _, err := p.consumer.NuclioClientSet.UpdateNuclioFunction(ctx, namespace, function); err != nil {
		return errors.Wrap(err, "Failed to update function")
	}

	p.Logger.InfoWithCtx(ctx, "Successfully migrated function authentication",
		"functionName", function.Name, "authentication mode", migration.mode)
	return nil
}

// setFunctionAuthentication writes the resolved mode onto the function's HTTP trigger.
func (p *Platform) setFunctionAuthentication(ctx context.Context,
	function *nuclioio.NuclioFunction,
	migration *functionAuthMigration) error {
	if migration.mode != auth.AuthenticationModeBasicAuth {
		return setHTTPTriggerAuthentication(&function.Spec, migration.mode, nil)
	}

	basicAuth, err := p.restoreAPIGatewayBasicAuth(ctx, migration.basicAuthAPIGateway)
	if err != nil {
		return errors.Wrap(err, "Failed to restore api gateway basic-auth credentials")
	}

	scrubber := p.GetFunctionScrubber()
	if !p.shouldScrubFunctionConfig(function, scrubber) {
		return setHTTPTriggerAuthentication(&function.Spec, migration.mode, basicAuth)
	}

	return p.setScrubbedFunctionBasicAuth(ctx, function, scrubber, basicAuth)
}

// setScrubbedFunctionBasicAuth writes the credentials through the function scrubber, so the password lands
// in the function's own secret and the spec holds only a reference to it - the same shape a normal deploy
// produces, and the only one the auth-proxy sidecar can restore.
func (p *Platform) setScrubbedFunctionBasicAuth(ctx context.Context,
	function *nuclioio.NuclioFunction,
	scrubber *functionconfig.Scrubber,
	basicAuth *platform.BasicAuth) error {
	functionConfig := &functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:      function.Name,
			Namespace: function.Namespace,
			Labels:    function.Labels,
		},
		Spec: function.Spec,
	}

	// restore first: scrubbing rewrites the function secret with only what it scrubbed in this pass, so any
	// other sensitive field left as a $ref would be lost
	restoredFunctionConfig, err := scrubber.RestoreFunctionConfig(ctx, functionConfig, common.KubePlatformName)
	if err != nil {
		return errors.Wrap(err, "Failed to restore function config")
	}

	if err := setHTTPTriggerAuthentication(&restoredFunctionConfig.Spec,
		auth.AuthenticationModeBasicAuth,
		basicAuth); err != nil {
		return err
	}

	scrubbedFunctionConfig, err := scrubber.ScrubFunctionConfig(ctx, restoredFunctionConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to scrub function config")
	}

	if scrubbedFunctionConfig == nil {
		return errors.New("Scrubber returned nil function config")
	}

	function.Spec = scrubbedFunctionConfig.Spec
	return nil
}

// restoreAPIGatewayBasicAuth reads the gateway's basic-auth credentials in plaintext, since in the CRD the
// password is only a $ref into the gateway's own secret.
func (p *Platform) restoreAPIGatewayBasicAuth(ctx context.Context,
	apiGateway *nuclioio.NuclioAPIGateway) (*platform.BasicAuth, error) {
	if apiGateway == nil {
		return nil, errors.New("No api gateway to read basic-auth credentials from")
	}

	apiGatewayConfig := &platform.APIGatewayConfig{
		Meta: platform.APIGatewayMeta{
			Name:      apiGateway.Name,
			Namespace: apiGateway.Namespace,
			Labels:    apiGateway.Labels,
		},
		Spec: apiGateway.Spec,
	}
	restoredAPIGatewayConfig, err := p.GetAPIGatewayScrubber().RestoreAPIGatewayConfig(ctx, apiGatewayConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to restore api gateway config (name=%s)", apiGateway.Name)
	}

	if restoredAPIGatewayConfig.Spec.Authentication == nil ||
		restoredAPIGatewayConfig.Spec.Authentication.BasicAuth == nil {
		return nil, errors.Errorf("Api gateway has no basic-auth credentials (name=%s)", apiGateway.Name)
	}

	return restoredAPIGatewayConfig.Spec.Authentication.BasicAuth, nil
}

// migrateAPIGateways is step 3: it drains the authentication configuration off the api gateway CRDs and
// re-provisions them, so the controller re-renders their ingresses without the nginx auth annotations. A
// gateway whose functions did not all migrate is skipped: draining it would leave those functions
// unauthenticated, and the next run could no longer derive their mode from it.
func (p *Platform) migrateAPIGateways(ctx context.Context,
	namespace string,
	apiGateways []nuclioio.NuclioAPIGateway,
	functionMigrations map[string]*functionAuthMigration) {

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(authMigrationConcurrency)
	for _, apiGateway := range apiGateways {
		unmigratedFunctionName := ""
		for _, upstream := range apiGateway.Spec.Upstreams {
			if upstream.NuclioFunction == nil {
				continue
			}
			if migration, found := functionMigrations[upstream.NuclioFunction.Name]; found &&
				!migration.migrated {
				// the function behind this gateway did not migrate, so skip the gateway and leave its authentication
				unmigratedFunctionName = upstream.NuclioFunction.Name
				break
			}
		}
		if unmigratedFunctionName != "" {
			// left unlabeled, so the next restart lists it again and re-migrates it.
			p.Logger.WarnWithCtx(ctx,
				"Api gateway fronts a function that did not migrate, skipping the gateway migration",
				"apiGatewayName", apiGateway.Name,
				"functionName", unmigratedFunctionName)
			continue
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			p.Logger.WarnWithCtx(ctx, "Failed to acquire a migration slot, stopping api gateway migration",
				"err", err.Error())
			break
		}

		wg.Go(func() {
			defer sem.Release(1)
			if err := p.migrateAPIGateway(ctx, namespace, &apiGateway); err != nil {
				p.Logger.WarnWithCtx(ctx, "Failed to migrate api gateway authentication",
					"apiGatewayName", apiGateway.Name,
					"err", err.Error())
			}
		})
	}
	wg.Wait()
}

func (p *Platform) migrateAPIGateway(ctx context.Context,
	namespace string,
	apiGateway *nuclioio.NuclioAPIGateway) error {
	p.Logger.InfoWithCtx(ctx, "Migrating api gateway authentication",
		"apiGatewayName", apiGateway.Name,
		"authenticationMode", apiGateway.Spec.AuthenticationMode)
	hadAuthentication := apiGateway.Spec.AuthenticationMode != "" || apiGateway.Spec.Authentication != nil

	// only a basicAuth gateway ever put anything in its config secret, so only that one needs cleaning up
	hadBasicAuth := apiGateway.Spec.Authentication != nil && apiGateway.Spec.Authentication.BasicAuth != nil

	apiGateway.Spec.AuthenticationMode = ""
	apiGateway.Spec.Authentication = nil
	markMigratedToFunctionAuthentication(apiGateway)

	// the api gateway operator only acts on a gateway in waitingForProvisioning, so a spec write alone would
	// leave a ready gateway's ingress still carrying the nginx auth annotations this migration is draining.
	// Ask for a re-provision the same way UpdateAPIGateway does, and only if there is something to re-render
	if hadAuthentication {
		apiGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning
	}

	if _, err := p.consumer.NuclioClientSet.UpdateNuclioAPIGateway(ctx, namespace, apiGateway); err != nil {
		return errors.Wrap(err, "Failed to update api gateway")
	}

	if hadBasicAuth {
		// the password now lives on the function, so delete only after the spec write, when nothing points
		// at the secret anymore. A failure here leaves an orphan secret, which goes away with the gateway
		if err := p.deleteAPIGatewayConfigSecret(ctx, namespace, apiGateway.Name); err != nil {
			return errors.Wrap(err, "Failed to delete api gateway config secret")
		}
	}

	p.Logger.InfoWithCtx(ctx, "Migrated api gateway authentication", "apiGatewayName", apiGateway.Name)
	return nil
}

func (p *Platform) deleteAPIGatewayConfigSecret(ctx context.Context, namespace, apiGatewayName string) error {
	// a gateway can own two secrets under the same apigateway-name label - this config secret and the nginx
	// basic-auth one the ingress renders - and only GetObjectSecret tells them apart by type
	secret, err := p.GetAPIGatewayScrubber().GetObjectSecret(ctx, apiGatewayName, namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to resolve the api gateway's config secret (name=%s)", apiGatewayName)
	}
	if secret == nil {
		return nil
	}

	p.Logger.DebugWithCtx(ctx, "Deleting migrated api gateway config secret",
		"apiGatewayName", apiGatewayName,
		"secretName", secret.Name)
	if err := p.consumer.KubeClientSet.DeleteSecret(ctx, namespace, secret.Name); err != nil {
		return errors.Wrapf(err, "Failed to delete secret %s", secret.Name)
	}
	return nil
}

// checkNotMigratedToFunctionAuthentication rejects a user write on a resource the migration has not marked
// yet, because the migration still owns it and one of the two writes would be silently lost.
func (p *Platform) checkNotMigratedToFunctionAuthentication(kind, name string, labels map[string]string) error {
	if !p.IsFunctionAuthenticationEnabled() ||
		labels[common.NuclioLabelKeyMigrationFunctionAuth] == common.NuclioLabelValueMigrationApplied {
		return nil
	}
	return nuclio.NewErrPreconditionFailed(fmt.Sprintf(
		"cannot be modified until its authentication migration completes (resource=%s) (name=%s)", kind, name))
}

// stampMigratedToFunctionAuthentication marks a resource created while function-level authentication is
// already on: it is created with the feature, so there is nothing to migrate on it.
func (p *Platform) stampMigratedToFunctionAuthentication(labels map[string]string) map[string]string {
	if !p.IsFunctionAuthenticationEnabled() {
		return labels
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[common.NuclioLabelKeyMigrationFunctionAuth] = common.NuclioLabelValueMigrationApplied
	return labels
}

// preserveMigrationLabel carries the migration label from the stored resource into an update request that
// does not echo it back. Needed for api gateways only - the function update paths never replace the label
// map.
func preserveMigrationLabel(requested, stored map[string]string) map[string]string {
	storedValue, found := stored[common.NuclioLabelKeyMigrationFunctionAuth]
	if !found {
		return requested
	}
	if requested == nil {
		requested = map[string]string{}
	}
	requested[common.NuclioLabelKeyMigrationFunctionAuth] = storedValue
	return requested
}

func markMigratedToFunctionAuthentication(resource metav1.Object) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[common.NuclioLabelKeyMigrationFunctionAuth] = common.NuclioLabelValueMigrationApplied
	resource.SetLabels(labels)
}

// translateAPIGatewayAuthenticationMode maps an api gateway's ingress-level authentication mode onto the
// matching function-level one. The split follows what the ingress does today: a mode that sets auth-signin
// redirects the browser, one that only sets auth-url returns 401.
func translateAPIGatewayAuthenticationMode(
	apiGateway *nuclioio.NuclioAPIGateway) (auth.AuthenticationMode, error) {

	switch apiGateway.Spec.AuthenticationMode {
	case "", auth.AuthenticationModeNone:
		return auth.AuthenticationModeNone, nil
	case auth.AuthenticationModeIguazio:
		return auth.AuthenticationModeBrowser, nil
	case auth.AuthenticationModeAccessKey:
		return auth.AuthenticationModeAPI, nil
	case auth.AuthenticationModeOauth2:
		if apiGateway.Spec.Authentication != nil &&
			apiGateway.Spec.Authentication.DexAuth != nil &&
			apiGateway.Spec.Authentication.DexAuth.RedirectUnauthorizedToSignIn {
			return auth.AuthenticationModeBrowser, nil
		}
		return auth.AuthenticationModeAPI, nil
	case auth.AuthenticationModeBasicAuth:
		return auth.AuthenticationModeBasicAuth, nil
	default:
		return "", errors.Errorf("Unknown api gateway authentication mode: %s",
			apiGateway.Spec.AuthenticationMode)
	}
}

// setHTTPTriggerAuthentication writes the mode onto the function's single HTTP trigger.
func setHTTPTriggerAuthentication(functionSpec *functionconfig.Spec,
	mode auth.AuthenticationMode,
	basicAuth *platform.BasicAuth) error {

	httpTrigger, err := functionconfig.GetHTTPTrigger(functionSpec.Triggers)
	if err != nil {
		return errors.Wrap(err, "Failed to get the function's HTTP trigger")
	}

	// the trigger map is keyed by the trigger's name, so an unnamed trigger would be written back under an
	// empty key and orphan the real one
	if httpTrigger.Name == "" {
		return errors.New("The function's HTTP trigger has no name")
	}

	if httpTrigger.Attributes == nil {
		httpTrigger.Attributes = map[string]interface{}{}
	}
	httpTrigger.Attributes[auth.AttributeAuthenticationMode] = string(mode)
	if basicAuth != nil {
		httpTrigger.Attributes[auth.AttributeAuthentication] = map[string]interface{}{
			auth.AttributeBasicAuth: map[string]interface{}{
				"username": basicAuth.Username,
				"password": basicAuth.Password,
			},
		}
	}

	functionSpec.Triggers[httpTrigger.Name] = httpTrigger
	return nil
}

// shouldScrubFunctionConfig mirrors the deploy path's masking decision, so the migration stores the
// basic-auth password exactly the way a normal deploy would.
func (p *Platform) shouldScrubFunctionConfig(function *nuclioio.NuclioFunction,
	scrubber *functionconfig.Scrubber) bool {
	return p.GetConfig().SensitiveFields.MaskSensitiveFields &&
		!function.Spec.DisableSensitiveFieldsMasking &&
		scrubber != nil
}

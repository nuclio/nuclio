/*
Copyright 2014 The Kubernetes Authors.

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

package validation

import (
	"fmt"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TODO: delete this global variable when we enable the validation of common
// fields by default.
var RepairMalformedUpdates bool = true

const FieldImmutableErrorMsg string = `field is immutable`

const totalAnnotationSizeLimitB int = 256 * (1 << 10) // 256 kB

// BannedOwners is a black list of object that are not allowed to be owners.
var BannedOwners = map[schema.GroupVersionKind]struct{}{
	schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Event"}: {},
}

// ValidateClusterName can be used to check whether the given cluster name is valid.
var ValidateClusterName = NameIsDNS1035Label

// ValidateAnnotations validates that a set of annotations are correctly defined.
func ValidateAnnotations(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	var totalSize int64
	for k, v := range annotations {
		for _, msg := range validation.IsQualifiedName(strings.ToLower(k)) {
			allErrs = append(allErrs, field.Invalid(fldPath, k, msg))
		}
		totalSize += (int64)(len(k)) + (int64)(len(v))
	}
	if totalSize > (int64)(totalAnnotationSizeLimitB) {
		allErrs = append(allErrs, field.TooLong(fldPath, "", totalAnnotationSizeLimitB))
	}
	return allErrs
}

func validateOwnerReference(ownerReference metav1.OwnerReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	gvk := schema.FromAPIVersionAndKind(ownerReference.APIVersion, ownerReference.Kind)
	// gvk.Group is empty for the legacy group.
	if len(gvk.Version) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("apiVersion"), ownerReference.APIVersion, "version must not be empty"))
	}
	if len(gvk.Kind) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), ownerReference.Kind, "kind must not be empty"))
	}
	if len(ownerReference.Name) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ownerReference.Name, "name must not be empty"))
	}
	if len(ownerReference.UID) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("uid"), ownerReference.UID, "uid must not be empty"))
	}
	if _, ok := BannedOwners[gvk]; ok {
		allErrs = append(allErrs, field.Invalid(fldPath, ownerReference, fmt.Sprintf("%s is disallowed from being an owner", gvk)))
	}
	return allErrs
}

func ValidateOwnerReferences(ownerReferences []metav1.OwnerReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	controllerName := ""
	for _, ref := range ownerReferences {
		allErrs = append(allErrs, validateOwnerReference(ref, fldPath)...)
		if ref.Controller != nil && *ref.Controller {
			if controllerName != "" {
				allErrs = append(allErrs, field.Invalid(fldPath, ownerReferences,
					fmt.Sprintf("Only one reference can have Controller set to true. Found \"true\" in references for %v and %v", controllerName, ref.Name)))
			} else {
				controllerName = ref.Name
			}
		}
	}
	return allErrs
}

// Validate finalizer names
func ValidateFinalizerName(stringValue string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsQualifiedName(stringValue) {
		allErrs = append(allErrs, field.Invalid(fldPath, stringValue, msg))
	}

	return allErrs
}

func ValidateNoNewFinalizers(newFinalizers []string, oldFinalizers []string, fldPath *field.Path) field.ErrorList {
	const newFinalizersErrorMsg string = `no new finalizers can be added if the object is being deleted`
	allErrs := field.ErrorList{}
	extra := sets.NewString(newFinalizers...).Difference(sets.NewString(oldFinalizers...))
	if len(extra) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("no new finalizers can be added if the object is being deleted, found new finalizers %#v", extra.List())))
	}
	return allErrs
}

func ValidateImmutableField(newVal, oldVal interface{}, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if !apiequality.Semantic.DeepEqual(oldVal, newVal) {
		allErrs = append(allErrs, field.Invalid(fldPath, newVal, FieldImmutableErrorMsg))
	}
	return allErrs
}

// ValidateObjectMeta validates an object's metadata on creation. It expects that name generation has already
// been performed.
// It doesn't return an error for rootscoped resources with namespace, because namespace should already be cleared before.
func ValidateObjectMeta(meta *metav1.ObjectMeta, requiresNamespace bool, nameFn ValidateNameFunc, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(meta.GenerateName) != 0 {
		for _, msg := range nameFn(meta.GenerateName, true) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("generateName"), meta.GenerateName, msg))
		}
	}
	// If the generated name validates, but the calculated value does not, it's a problem with generation, and we
	// report it here. This may confuse users, but indicates a programming bug and still must be validated.
	// If there are multiple fields out of which one is required then add an or as a separator
	if len(meta.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name or generateName is required"))
	} else {
		for _, msg := range nameFn(meta.Name, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), meta.Name, msg))
		}
	}
	if requiresNamespace {
		if len(meta.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
		} else {
			for _, msg := range ValidateNamespaceName(meta.Namespace, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), meta.Namespace, msg))
			}
		}
	} else {
		if len(meta.Namespace) != 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("namespace"), "not allowed on this type"))
		}
	}
	if len(meta.ClusterName) != 0 {
		for _, msg := range ValidateClusterName(meta.ClusterName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterName"), meta.ClusterName, msg))
		}
	}
	allErrs = append(allErrs, ValidateNonnegativeField(meta.Generation, fldPath.Child("generation"))...)
	allErrs = append(allErrs, v1validation.ValidateLabels(meta.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, ValidateAnnotations(meta.Annotations, fldPath.Child("annotations"))...)
	allErrs = append(allErrs, ValidateOwnerReferences(meta.OwnerReferences, fldPath.Child("ownerReferences"))...)
	allErrs = append(allErrs, ValidateFinalizers(meta.Finalizers, fldPath.Child("finalizers"))...)
	return allErrs
}

// ValidateFinalizers tests if the finalizers name are valid, and if there are conflicting finalizers.
func ValidateFinalizers(finalizers []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	hasFinalizerOrphanDependents := false
	hasFinalizerDeleteDependents := false
	for _, finalizer := range finalizers {
		allErrs = append(allErrs, ValidateFinalizerName(finalizer, fldPath)...)
		if finalizer == metav1.FinalizerOrphanDependents {
			hasFinalizerOrphanDependents = true
		}
		if finalizer == metav1.FinalizerDeleteDependents {
			hasFinalizerDeleteDependents = true
		}
	}
	if hasFinalizerDeleteDependents && hasFinalizerOrphanDependents {
		allErrs = append(allErrs, field.Invalid(fldPath, finalizers, fmt.Sprintf("finalizer %s and %s cannot be both set", metav1.FinalizerOrphanDependents, metav1.FinalizerDeleteDependents)))
	}
	return allErrs
}

// ValidateObjectMetaUpdate validates an object's metadata when updated
func ValidateObjectMetaUpdate(newMeta, oldMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !RepairMalformedUpdates && newMeta.UID != oldMeta.UID {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("uid"), newMeta.UID, "field is immutable"))
	}
	// in the event it is left empty, set it, to allow clients more flexibility
	// TODO: remove the following code that repairs the update request when we retire the clients that modify the immutable fields.
	// Please do not copy this pattern elsewhere; validation functions should not be modifying the objects they are passed!
	if RepairMalformedUpdates {
		if len(newMeta.UID) == 0 {
			newMeta.UID = oldMeta.UID
		}
		// ignore changes to timestamp
		if oldMeta.CreationTimestamp.IsZero() {
			oldMeta.CreationTimestamp = newMeta.CreationTimestamp
		} else {
			newMeta.CreationTimestamp = oldMeta.CreationTimestamp
		}
		// an object can never remove a deletion timestamp or clear/change grace period seconds
		if !oldMeta.DeletionTimestamp.IsZero() {
			newMeta.DeletionTimestamp = oldMeta.DeletionTimestamp
		}
		if oldMeta.DeletionGracePeriodSeconds != nil && newMeta.DeletionGracePeriodSeconds == nil {
			newMeta.DeletionGracePeriodSeconds = oldMeta.DeletionGracePeriodSeconds
		}
	}

	// TODO: needs to check if newMeta==nil && oldMeta !=nil after the repair logic is removed.
	if newMeta.DeletionGracePeriodSeconds != nil && (oldMeta.DeletionGracePeriodSeconds == nil || *newMeta.DeletionGracePeriodSeconds != *oldMeta.DeletionGracePeriodSeconds) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("deletionGracePeriodSeconds"), newMeta.DeletionGracePeriodSeconds, "field is immutable; may only be changed via deletion"))
	}
	if newMeta.DeletionTimestamp != nil && (oldMeta.DeletionTimestamp == nil || !newMeta.DeletionTimestamp.Equal(*oldMeta.DeletionTimestamp)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("deletionTimestamp"), newMeta.DeletionTimestamp, "field is immutable; may only be changed via deletion"))
	}

	// Finalizers cannot be added if the object is already being deleted.
	if oldMeta.DeletionTimestamp != nil {
		allErrs = append(allErrs, ValidateNoNewFinalizers(newMeta.Finalizers, oldMeta.Finalizers, fldPath.Child("finalizers"))...)
	}

	// Reject updates that don't specify a resource version
	if len(newMeta.ResourceVersion) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resourceVersion"), newMeta.ResourceVersion, "must be specified for an update"))
	}

	// Generation shouldn't be decremented
	if newMeta.Generation < oldMeta.Generation {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("generation"), newMeta.Generation, "must not be decremented"))
	}

	allErrs = append(allErrs, ValidateImmutableField(newMeta.Name, oldMeta.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, ValidateImmutableField(newMeta.Namespace, oldMeta.Namespace, fldPath.Child("namespace"))...)
	allErrs = append(allErrs, ValidateImmutableField(newMeta.UID, oldMeta.UID, fldPath.Child("uid"))...)
	allErrs = append(allErrs, ValidateImmutableField(newMeta.CreationTimestamp, oldMeta.CreationTimestamp, fldPath.Child("creationTimestamp"))...)
	allErrs = append(allErrs, ValidateImmutableField(newMeta.ClusterName, oldMeta.ClusterName, fldPath.Child("clusterName"))...)

	allErrs = append(allErrs, v1validation.ValidateLabels(newMeta.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, ValidateAnnotations(newMeta.Annotations, fldPath.Child("annotations"))...)
	allErrs = append(allErrs, ValidateOwnerReferences(newMeta.OwnerReferences, fldPath.Child("ownerReferences"))...)

	return allErrs
}

package project

import (
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type SafeCacheTestSuite struct {
	suite.Suite

	logger    logger.Logger
	safeCache *SafeCache
}

func (suite *SafeCacheTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.safeCache = NewSafeCache()
}

func (suite *SafeCacheTestSuite) TestAdd() {
	projectInstance := suite.createProjectInstance("test-name", "test-namespace")

	suite.safeCache.Add(projectInstance)
	projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace: "test-namespace", Name: "test-name"}})
	suite.Require().Equal(projects, []platform.Project{projectInstance})
}


func (suite *SafeCacheTestSuite) TestAddMany() {
	projectInstances := []platform.Project{}

	for i := 0; i < 10; i++ {
		testName := fmt.Sprintf("test-name-%d", i)
		projectInstances = append(projectInstances, suite.createProjectInstance(testName, "test-namespace"))
	}

	suite.safeCache.AddMany(projectInstances)

	projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace:"test-namespace"}})
	suite.Require().Equal(len(projects), 10)

	for _, projectInstance := range projectInstances {
		projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta: projectInstance.GetConfig().Meta})
		suite.Require().Equal(projects, []platform.Project{projectInstance})
	}
}


func (suite *SafeCacheTestSuite) TestUpdate() {
	projectInstance := suite.createProjectInstance("test-name", "test-namespace")

	// add project
	suite.safeCache.Add(projectInstance)
	projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace: "test-namespace", Name: "test-name"}})
	suite.Require().Equal(projects[0].GetConfig().Spec.Description, "default-description")

	// update project
	projectInstance.ProjectConfig.Spec.Description = "new-description"
	suite.safeCache.Update(projectInstance)
	projects = suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace: "test-namespace", Name: "test-name"}})
	suite.Require().Equal(projects[0].GetConfig().Spec.Description, "new-description")
}

func (suite *SafeCacheTestSuite) TestDelete() {
	projectInstance := suite.createProjectInstance("test-name", "test-namespace")

	// add project
	suite.safeCache.Add(projectInstance)
	projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace: "test-namespace", Name: "test-name"}})
	suite.Require().Equal(projects, []platform.Project{projectInstance})

	// delete project
	suite.safeCache.Delete("test-namespace", "test-name")
	projects = suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace: "test-namespace", Name: "test-name"}})
	suite.Require().Equal(projects, []platform.Project{})
}

func (suite *SafeCacheTestSuite) TestGet() {
	projectInstancesByNamespace := map[string][]platform.Project{}

	namespaces := []string{}
	for i := 0; i < 10; i++ {
		namespaces = append(namespaces, fmt.Sprintf("namespace-%d", i))
	}

	for _, namespace := range namespaces {
		for i := 0; i < 10; i++ {
			testName := fmt.Sprintf("test-name-%d", i)
			projectInstancesByNamespace[namespace] = append(projectInstancesByNamespace[namespace], suite.createProjectInstance(testName, namespace))
		}
		suite.safeCache.AddMany(projectInstancesByNamespace[namespace])
	}

	for _, namespace := range namespaces {
		projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta:platform.ProjectMeta{Namespace:namespace}})
		suite.Require().Equal(len(projects), 10)

		for _, projectInstanceInNamespace := range projectInstancesByNamespace[namespace] {
			projects := suite.safeCache.Get(&platform.GetProjectsOptions{Meta: projectInstanceInNamespace.GetConfig().Meta})
			suite.Require().Equal(projects, []platform.Project{projectInstanceInNamespace})
		}
	}
}

func (suite *SafeCacheTestSuite) createProjectInstance(name, namespace string) *platform.AbstractProject {
	return &platform.AbstractProject{
		ProjectConfig: platform.ProjectConfig{
			Meta:platform.ProjectMeta{
				Name: name,
				Namespace: namespace,
			},
			Spec:platform.ProjectSpec{
				Description: "default-description",
			},
		},
	}
}

func TestSafeCacheTestSuite(t *testing.T) {
	suite.Run(t, new(SafeCacheTestSuite))
}

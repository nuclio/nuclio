package functiondep

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/logger"

	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Client struct {
	logger logger.Logger
	clientSet  *kubernetes.Clientset
}

func NewClient(parentLogger logger.Logger,
	clientSet *kubernetes.Clientset) (*Client, error) {

	newClient := &Client{
		logger:    parentLogger.GetChild("functiondep").(logger.Logger),
		clientSet: clientSet,
	}

	return newClient, nil
}

func (c *Client) List(namespace string) ([]v1beta1.Deployment, error) {
	classLabelKey, classLabelValue := c.getClassLabels()
	listOptions := meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", classLabelKey, classLabelValue),
	}

	result, err := c.clientSet.AppsV1beta1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list deployments")
	}

	c.logger.DebugWith("Got deployments", "num", len(result.Items))

	return result.Items, nil
}

func (c *Client) Get(namespace string, name string) (*v1beta1.Deployment, error) {
	var result *v1beta1.Deployment

	result, err := c.clientSet.AppsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
	c.logger.DebugWith("Got deployment",
		"namespace", namespace,
		"name", name,
		"result", result,
		"err", err)

	// if we didn't find it
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return result, err
}

func (c *Client) getClassLabels() (string, string) {
	return "serverless", "nuclio"
}
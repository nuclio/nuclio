package kubernetes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/build/kubernetes"
	"golang.org/x/build/kubernetes/api"
)

type handlers []func(w http.ResponseWriter, r *http.Request) error

func newTestPod() *api.Pod {
	return &api.Pod{
		TypeMeta: api.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: api.ObjectMeta{
			Name: "test-pod",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  "test-container",
					Image: "test-image:latest",
				},
			},
		},
	}
}

func (hs *handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(*hs) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unexpected request: %v", r)
		return
	}
	h := (*hs)[0]
	*hs = (*hs)[1:]
	if err := h(w, r); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unexpected error: %v", err)
		return
	}
}

func TestRunPod(t *testing.T) {
	hs := handlers{
		func(w http.ResponseWriter, r *http.Request) error {
			if r.Method != http.MethodPost {
				return fmt.Errorf("expected %q, got %q", http.MethodPost, r.Method)

			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(newTestPod())
			return nil
		},
		func(w http.ResponseWriter, r *http.Request) error {
			if r.Method != http.MethodGet {
				return fmt.Errorf("expected %q, got %q", http.MethodGet, r.Method)
			}
			w.WriteHeader(http.StatusOK)
			readyPod := newTestPod()
			readyPod.Status.Phase = api.PodRunning
			json.NewEncoder(w).Encode(readyPod)
			return nil
		},
	}
	s := httptest.NewServer(&hs)
	defer s.Close()

	c, err := kubernetes.NewClient(s.URL, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ps, err := c.RunLongLivedPod(context.Background(), newTestPod())
	if err != nil {
		t.Fatalf("RunLongLivePod: %v", err)
	}
	if ps.Phase != api.PodRunning {
		t.Fatalf("Pod phase = %q; want %q", ps.Phase, api.PodRunning)
	}
	if len(hs) != 0 {
		t.Fatalf("failed to process all expected requests: %d left", len(hs))
	}
}

package watches

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Watch struct {
	GroupVersionKind        schema.GroupVersionKind
	Chart                   *chart.Chart `json:"chart"`
	WatchDependentResources bool
	OverrideValues          map[string]string
}

type yamlWatch struct {
	Group                   string            `yaml:"group"`
	Version                 string            `yaml:"version"`
	Kind                    string            `yaml:"kind"`
	Chart                   string            `yaml:"chart"`
	WatchDependentResources bool              `yaml:"watchDependentResources"`
	OverrideValues          map[string]string `yaml:"overrideValues"`
}

func (w *yamlWatch) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// by default, the operator will watch dependent resources
	w.WatchDependentResources = true

	// hide watch data in plain struct to prevent unmarshal from calling
	// UnmarshalYAML again
	type plain yamlWatch

	return unmarshal((*plain)(w))
}

// Load loads a slice of Watches from the watch file at `path`. For each entry
// in the watches file, it verifies the configuration. If an error is
// encountered loading the file or verifying the configuration, it will be
// returned.
func Load(path string) ([]Watch, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlWatches := []yamlWatch{}
	err = yaml.Unmarshal(b, &yamlWatches)
	if err != nil {
		return nil, err
	}

	watches := []Watch{}
	watchesMap := make(map[schema.GroupVersionKind]Watch)
	for _, w := range yamlWatches {
		gvk := schema.GroupVersionKind{
			Group:   w.Group,
			Version: w.Version,
			Kind:    w.Kind,
		}

		if err := verifyGVK(gvk); err != nil {
			return nil, fmt.Errorf("invalid GVK: %s: %w", gvk, err)
		}

		cl, err := loader.Load(w.Chart)
		if err != nil {
			return nil, fmt.Errorf("invalid chart %s: %w", w.Chart, err)
		}

		if _, ok := watchesMap[gvk]; ok {
			return nil, fmt.Errorf("duplicate GVK: %s", gvk)
		}

		watch := Watch{
			GroupVersionKind:        gvk,
			Chart:                   cl,
			WatchDependentResources: w.WatchDependentResources,
			OverrideValues:          expandOverrideEnvs(w.OverrideValues),
		}
		watchesMap[gvk] = watch
		watches = append(watches, watch)
	}
	return watches, nil
}

func expandOverrideEnvs(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = os.ExpandEnv(v)
	}
	return out
}

func verifyGVK(gvk schema.GroupVersionKind) error {
	// A GVK without a group is valid. Certain scenarios may cause a GVK
	// without a group to fail in other ways later in the initialization
	// process.
	if gvk.Version == "" {
		return errors.New("version must not be empty")
	}
	if gvk.Kind == "" {
		return errors.New("kind must not be empty")
	}
	return nil
}

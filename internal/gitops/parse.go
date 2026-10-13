package gitops

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Richonn/driftwatch/internal/cluster"
	"gopkg.in/yaml.v3"
)

func isCustomAPIVersion(apiVersion string) bool {
	return !cluster.IsBuiltinAPIVersion(apiVersion)
}

type manifest struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   manifestMeta           `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
	Data       map[string]string      `yaml:"data"`
	Type       string                 `yaml:"type"`
}

type manifestMeta struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
}

var supportedKinds = map[string]bool{
	"Deployment":     true,
	"StatefulSet":    true,
	"DaemonSet":      true,
	"Service":        true,
	"ConfigMap":      true,
	"Secret":         true,
	"Ingress":        true,
	"ServiceAccount": true,
	"NetworkPolicy":  true,
}

func ParseManifests(rootDir, subPath string, helmOpts HelmOptions) ([]cluster.Resource, error) {
	searchRoot := filepath.Join(rootDir, subPath)
	var resources []cluster.Resource

	err := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if path != searchRoot {
				if isHelmChart(path) {
					found, err := renderChart(path, helmOpts)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warn: skipping helm chart %s: %v\n", path, err)
					} else {
						resources = append(resources, found...)
					}
					return filepath.SkipDir
				}
				if isKustomize(path) {
					found, err := renderKustomize(path)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warn: skipping kustomize dir %s: %v\n", path, err)
					} else {
						resources = append(resources, found...)
					}
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		found, err := parseFile(path, helmOpts.IncludeCRDs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skipping %s: %v\n", path, err)
			return nil
		}
		resources = append(resources, found...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", searchRoot, err)
	}
	return resources, nil
}

func parseFile(path string, includeCRDs bool) ([]cluster.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(filepath.Base(path), "Chart.yaml") {
		fmt.Fprintf(os.Stderr, "warn: Helm chart detected at %s — use --helm-values or place chart in a subdirectory\n", path)
		return nil, nil
	}

	return parseYAMLDocs(bytes.NewReader(data), includeCRDs)
}

func parseYAMLDocs(r io.Reader, includeCRDs bool) ([]cluster.Resource, error) {
	var resources []cluster.Resource
	decoder := yaml.NewDecoder(r)

	for {
		var m manifest
		err := decoder.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if m.Kind == "" || m.Metadata.Name == "" {
			continue
		}
		if !supportedKinds[m.Kind] {
			if !includeCRDs || !isCustomAPIVersion(m.APIVersion) {
				continue
			}
		}

		r := cluster.Resource{
			Kind:      m.Kind,
			Name:      m.Metadata.Name,
			Namespace: m.Metadata.Namespace,
			Labels:    m.Metadata.Labels,
		}

		switch m.Kind {
		case "ConfigMap":
			r.Spec = map[string]interface{}{"data": m.Data}
		case "Secret":
			keys := make([]string, 0, len(m.Data))
			for k := range m.Data {
				keys = append(keys, k)
			}
			r.Spec = map[string]interface{}{"type": m.Type, "keys": keys}
		case "ServiceAccount":
			r.Spec = map[string]interface{}{}
		default:
			r.Spec = m.Spec
		}

		resources = append(resources, r)
	}
	return resources, nil
}

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

func ParseManifests(rootDir, subPath string) ([]cluster.Resource, error) {
	searchRoot := filepath.Join(rootDir, subPath)
	var resources []cluster.Resource

	err := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		found, err := parseFile(path)
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

func parseFile(path string) ([]cluster.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(filepath.Base(path), "Chart.yaml") {
		fmt.Fprintf(os.Stderr, "warn: Helm chart detected at %s — Helm rendering not supported in v1, skipping\n", path)
		return nil, nil
	}

	var resources []cluster.Resource
	decoder := yaml.NewDecoder(bytes.NewReader(data))

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
			continue
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

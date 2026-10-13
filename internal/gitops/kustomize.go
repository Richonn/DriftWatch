package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Richonn/driftwatch/internal/cluster"
	"gopkg.in/yaml.v3"
)

type kustomization struct {
	Resources  []string       `yaml:"resources"`
	Namespace  string         `yaml:"namespace"`
	NamePrefix string         `yaml:"namePrefix"`
	NameSuffix string         `yaml:"nameSuffix"`
	Images     []imageOverride `yaml:"images"`
}

type imageOverride struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
}

func isKustomize(dir string) bool {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func renderKustomize(dir string) ([]cluster.Resource, error) {
	k, err := loadKustomization(dir)
	if err != nil {
		return nil, err
	}

	imgIndex := buildImageIndex(k.Images)

	var resources []cluster.Resource
	for _, ref := range k.Resources {
		path := filepath.Join(dir, ref)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("kustomize resource %s: %w", ref, err)
		}
		var found []cluster.Resource
		if info.IsDir() {
			found, err = ParseManifests(path, ".", HelmOptions{})
		} else {
			found, err = parseFile(path)
		}
		if err != nil {
			return nil, fmt.Errorf("kustomize resource %s: %w", ref, err)
		}
		resources = append(resources, found...)
	}

	for i := range resources {
		r := &resources[i]
		if k.Namespace != "" {
			r.Namespace = k.Namespace
		}
		if k.NamePrefix != "" {
			r.Name = k.NamePrefix + r.Name
		}
		if k.NameSuffix != "" {
			r.Name = r.Name + k.NameSuffix
		}
		applyImageOverrides(r, imgIndex)
	}

	return resources, nil
}

func loadKustomization(dir string) (*kustomization, error) {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var k kustomization
		if err := yaml.Unmarshal(data, &k); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		return &k, nil
	}
	return nil, fmt.Errorf("no kustomization file found in %s", dir)
}

func buildImageIndex(overrides []imageOverride) map[string]imageOverride {
	idx := make(map[string]imageOverride, len(overrides))
	for _, img := range overrides {
		idx[img.Name] = img
	}
	return idx
}

func applyImageOverrides(r *cluster.Resource, idx map[string]imageOverride) {
	tmpl, _ := r.Spec["template"].(map[string]interface{})
	if tmpl == nil {
		return
	}
	podSpec, _ := tmpl["spec"].(map[string]interface{})
	if podSpec == nil {
		return
	}
	items, _ := podSpec["containers"].([]interface{})
	for _, c := range items {
		cm, _ := c.(map[string]interface{})
		if cm == nil {
			continue
		}
		img, _ := cm["image"].(string)
		if img == "" {
			continue
		}
		name := strings.SplitN(img, ":", 2)[0]
		if ov, ok := idx[name]; ok {
			newName := name
			if ov.NewName != "" {
				newName = ov.NewName
			}
			tag := strings.SplitN(img, ":", 2)
			newImg := newName
			if ov.NewTag != "" {
				newImg = newName + ":" + ov.NewTag
			} else if len(tag) == 2 {
				newImg = newName + ":" + tag[1]
			}
			cm["image"] = newImg
		}
	}
}

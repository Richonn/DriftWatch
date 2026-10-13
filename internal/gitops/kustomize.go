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
	Resources             []string          `yaml:"resources"`
	Namespace             string            `yaml:"namespace"`
	NamePrefix            string            `yaml:"namePrefix"`
	NameSuffix            string            `yaml:"nameSuffix"`
	Images                []imageOverride   `yaml:"images"`
	Patches               []kustomizePatch  `yaml:"patches"`
	PatchesStrategicMerge []string          `yaml:"patchesStrategicMerge"`
	PatchesJson6902       []json6902Entry   `yaml:"patchesJson6902"`
}

type imageOverride struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
}

type patchTarget struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type kustomizePatch struct {
	Path   string      `yaml:"path"`
	Patch  string      `yaml:"patch"`
	Target patchTarget `yaml:"target"`
}

type json6902Entry struct {
	Target patchTarget `yaml:"target"`
	Path   string      `yaml:"path"`
}

type jsonOp struct {
	Op    string      `yaml:"op"`
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value"`
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
			found, err = parseFile(path, false)
		}
		if err != nil {
			return nil, fmt.Errorf("kustomize resource %s: %w", ref, err)
		}
		resources = append(resources, found...)
	}

	if err := applyStrategicMergePatches(dir, k.PatchesStrategicMerge, resources); err != nil {
		return nil, err
	}
	for _, p := range k.Patches {
		if p.Path != "" {
			if err := applyStrategicMergePatches(dir, []string{p.Path}, resources); err != nil {
				return nil, err
			}
		} else if p.Patch != "" {
			var ops []jsonOp
			if err := yaml.Unmarshal([]byte(p.Patch), &ops); err != nil {
				return nil, fmt.Errorf("parse inline patch: %w", err)
			}
			for i := range resources {
				if matchesTarget(&resources[i], p.Target) {
					if err := applyJSONOps(&resources[i], ops); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	for _, entry := range k.PatchesJson6902 {
		data, err := os.ReadFile(filepath.Join(dir, entry.Path))
		if err != nil {
			return nil, fmt.Errorf("read json6902 patch %s: %w", entry.Path, err)
		}
		var ops []jsonOp
		if err := yaml.Unmarshal(data, &ops); err != nil {
			return nil, fmt.Errorf("parse json6902 patch %s: %w", entry.Path, err)
		}
		for i := range resources {
			if matchesTarget(&resources[i], entry.Target) {
				if err := applyJSONOps(&resources[i], ops); err != nil {
					return nil, err
				}
			}
		}
	}

	imgIndex := buildImageIndex(k.Images)
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

func matchesTarget(r *cluster.Resource, t patchTarget) bool {
	if t.Kind != "" && t.Kind != r.Kind {
		return false
	}
	if t.Name != "" && t.Name != r.Name {
		return false
	}
	if t.Namespace != "" && t.Namespace != r.Namespace {
		return false
	}
	return true
}

func filterByTarget(resources []cluster.Resource, t patchTarget) []cluster.Resource {
	var out []cluster.Resource
	for i := range resources {
		if matchesTarget(&resources[i], t) {
			out = append(out, resources[i])
		}
	}
	return out
}

func applyStrategicMergePatches(dir string, paths []string, resources []cluster.Resource) error {
	for _, patchPath := range paths {
		data, err := os.ReadFile(filepath.Join(dir, patchPath))
		if err != nil {
			return fmt.Errorf("read patch %s: %w", patchPath, err)
		}
		var patch map[string]interface{}
		if err := yaml.Unmarshal(data, &patch); err != nil {
			return fmt.Errorf("parse patch %s: %w", patchPath, err)
		}

		kind, _ := patch["kind"].(string)
		meta, _ := patch["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		ns, _ := meta["namespace"].(string)
		spec, _ := patch["spec"].(map[string]interface{})

		target := patchTarget{Kind: kind, Name: name, Namespace: ns}
		for i := range resources {
			if matchesTarget(&resources[i], target) && spec != nil {
				resources[i].Spec = deepMerge(resources[i].Spec, spec)
			}
		}
	}
	return nil
}

func deepMerge(base, patch map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, pv := range patch {
		if bv, ok := out[k]; ok {
			bMap, bOk := bv.(map[string]interface{})
			pMap, pOk := pv.(map[string]interface{})
			if bOk && pOk {
				out[k] = deepMerge(bMap, pMap)
				continue
			}
		}
		out[k] = pv
	}
	return out
}

func applyJSONOps(r *cluster.Resource, ops []jsonOp) error {
	for _, op := range ops {
		parts := splitJSONPointer(op.Path)
		switch op.Op {
		case "replace", "add":
			if err := jsonSet(r.Spec, parts, op.Value); err != nil {
				return fmt.Errorf("json patch %s %s: %w", op.Op, op.Path, err)
			}
		case "remove":
			jsonRemove(r.Spec, parts)
		}
	}
	return nil
}

func splitJSONPointer(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts
}

func jsonSet(m map[string]interface{}, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		m[parts[0]] = value
		return nil
	}
	next, ok := m[parts[0]].(map[string]interface{})
	if !ok {
		next = map[string]interface{}{}
		m[parts[0]] = next
	}
	return jsonSet(next, parts[1:], value)
}

func jsonRemove(m map[string]interface{}, parts []string) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		delete(m, parts[0])
		return
	}
	if next, ok := m[parts[0]].(map[string]interface{}); ok {
		jsonRemove(next, parts[1:])
	}
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

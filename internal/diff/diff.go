package diff

import (
	"fmt"
	"time"

	"github.com/Richonn/driftwatch/internal/cluster"
)

type DriftType string

const (
	DriftMissingInCluster DriftType = "MISSING_IN_CLUSTER"
	DriftMissingInGitOps  DriftType = "MISSING_IN_GITOPS"
	DriftSpecDrift        DriftType = "SPEC_DRIFT"
)

type DriftItem struct {
	Kind      string
	Name      string
	Namespace string
	DriftType DriftType
	Details   []string
}

type DriftReport struct {
	ScannedAt      time.Time
	ClusterContext string
	GitOpsRepo     string
	TotalResources int
	DriftCount     int
	Drifts         []DriftItem
}

type resourceKey struct {
	Kind      string
	Name      string
	Namespace string
}

func keyOf(r cluster.Resource) resourceKey {
	return resourceKey{Kind: r.Kind, Name: r.Name, Namespace: r.Namespace}
}

func Compare(liveResources, gitopsResources []cluster.Resource, clusterContext, gitopsRepo string) *DriftReport {
	report := &DriftReport{
		ScannedAt:      time.Now(),
		ClusterContext: clusterContext,
		GitOpsRepo:     gitopsRepo,
	}

	liveIndex := make(map[resourceKey]cluster.Resource, len(liveResources))
	for _, r := range liveResources {
		liveIndex[keyOf(r)] = r
	}
	gitopsIndex := make(map[resourceKey]cluster.Resource, len(gitopsResources))
	for _, r := range gitopsResources {
		gitopsIndex[keyOf(r)] = r
	}

	seen := make(map[resourceKey]bool)
	for k := range liveIndex {
		seen[k] = true
	}
	for k := range gitopsIndex {
		seen[k] = true
	}

	report.TotalResources = len(seen)

	for key := range seen {
		live, inLive := liveIndex[key]
		gitops, inGitOps := gitopsIndex[key]

		switch {
		case inLive && !inGitOps:
			report.Drifts = append(report.Drifts, DriftItem{
				Kind:      key.Kind,
				Name:      key.Name,
				Namespace: key.Namespace,
				DriftType: DriftMissingInGitOps,
				Details:   []string{"resource exists in cluster but has no manifest in GitOps repo"},
			})

		case !inLive && inGitOps:
			report.Drifts = append(report.Drifts, DriftItem{
				Kind:      key.Kind,
				Name:      key.Name,
				Namespace: key.Namespace,
				DriftType: DriftMissingInCluster,
				Details:   []string{"manifest defined in GitOps repo but resource not found in cluster"},
			})

		case inLive && inGitOps:
			if details := specDiff(live, gitops); len(details) > 0 {
				report.Drifts = append(report.Drifts, DriftItem{
					Kind:      key.Kind,
					Name:      key.Name,
					Namespace: key.Namespace,
					DriftType: DriftSpecDrift,
					Details:   details,
				})
			}
		}
	}

	report.DriftCount = len(report.Drifts)
	return report
}

func specDiff(live, gitops cluster.Resource) []string {
	var details []string

	switch live.Kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		details = append(details, diffReplicas(live.Spec, gitops.Spec)...)
		details = append(details, diffContainerImages(live.Spec, gitops.Spec)...)
		details = append(details, diffContainerResources(live.Spec, gitops.Spec)...)
		details = append(details, diffLabels(live.Labels, gitops.Labels)...)

	case "Service":
		details = append(details, diffField(live.Spec, gitops.Spec, "type", "spec.type")...)
		details = append(details, diffField(live.Spec, gitops.Spec, "clusterIP", "spec.clusterIP")...)

	case "ConfigMap":
		liveData := mapStr(live.Spec["data"])
		gitopsData := mapStr(gitops.Spec["data"])
		for k, lv := range liveData {
			if gv, ok := gitopsData[k]; !ok {
				details = append(details, fmt.Sprintf("data.%s: present in cluster, missing in GitOps", k))
			} else if lv != gv {
				details = append(details, fmt.Sprintf("data.%s: cluster=%q gitops=%q", k, lv, gv))
			}
		}
		for k := range gitopsData {
			if _, ok := liveData[k]; !ok {
				details = append(details, fmt.Sprintf("data.%s: missing in cluster, defined in GitOps", k))
			}
		}

	case "Ingress":
		details = append(details, diffField(live.Spec, gitops.Spec, "ingressClassName", "spec.ingressClassName")...)

	default:
		details = append(details, diffLabels(live.Labels, gitops.Labels)...)
	}

	return details
}

func diffReplicas(live, gitops map[string]interface{}) []string {
	lv := intField(live, "replicas")
	gv := intField(gitops, "replicas")
	if lv != nil && gv != nil && *lv != *gv {
		return []string{fmt.Sprintf("spec.replicas: cluster=%d gitops=%d", *lv, *gv)}
	}
	return nil
}

func diffContainerImages(live, gitops map[string]interface{}) []string {
	liveContainers := containers(live)
	gitopsContainers := containers(gitops)
	var details []string
	for name, limg := range liveContainers {
		if gimg, ok := gitopsContainers[name]; ok {
			if limg != gimg {
				details = append(details, fmt.Sprintf("container %q image: cluster=%s gitops=%s", name, limg, gimg))
			}
		}
	}
	return details
}

func diffContainerResources(live, gitops map[string]interface{}) []string {
	liveRes := containerResources(live)
	gitopsRes := containerResources(gitops)
	var details []string
	for name, lr := range liveRes {
		gr, ok := gitopsRes[name]
		if !ok {
			continue
		}
		if lr != gr {
			details = append(details, fmt.Sprintf("container %q resources: cluster=%s gitops=%s", name, lr, gr))
		}
	}
	return details
}

func diffLabels(live, gitops map[string]string) []string {
	var details []string
	for k, lv := range live {
		if gv, ok := gitops[k]; ok {
			if lv != gv {
				details = append(details, fmt.Sprintf("label %q: cluster=%s gitops=%s", k, lv, gv))
			}
		}
	}
	return details
}

func diffField(live, gitops map[string]interface{}, field, label string) []string {
	lv := fmt.Sprintf("%v", live[field])
	gv := fmt.Sprintf("%v", gitops[field])
	if lv != gv {
		return []string{fmt.Sprintf("%s: cluster=%s gitops=%s", label, lv, gv)}
	}
	return nil
}

func intField(spec map[string]interface{}, key string) *int64 {
	v, ok := spec[key]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case int:
		i := int64(n)
		return &i
	case int64:
		return &n
	case float64:
		i := int64(n)
		return &i
	}
	return nil
}

func containers(spec map[string]interface{}) map[string]string {
	result := map[string]string{}
	tmpl, _ := spec["template"].(map[string]interface{})
	if tmpl == nil {
		return result
	}
	podSpec, _ := tmpl["spec"].(map[string]interface{})
	if podSpec == nil {
		return result
	}
	items, _ := podSpec["containers"].([]interface{})
	for _, c := range items {
		cm, _ := c.(map[string]interface{})
		if cm == nil {
			continue
		}
		name, _ := cm["name"].(string)
		image, _ := cm["image"].(string)
		if name != "" {
			result[name] = image
		}
	}
	return result
}

func containerResources(spec map[string]interface{}) map[string]string {
	result := map[string]string{}
	tmpl, _ := spec["template"].(map[string]interface{})
	if tmpl == nil {
		return result
	}
	podSpec, _ := tmpl["spec"].(map[string]interface{})
	if podSpec == nil {
		return result
	}
	items, _ := podSpec["containers"].([]interface{})
	for _, c := range items {
		cm, _ := c.(map[string]interface{})
		if cm == nil {
			continue
		}
		name, _ := cm["name"].(string)
		if name == "" {
			continue
		}
		result[name] = fmt.Sprintf("%v", cm["resources"])
	}
	return result
}

func mapStr(v interface{}) map[string]string {
	m, _ := v.(map[string]string)
	if m != nil {
		return m
	}
	mi, _ := v.(map[interface{}]interface{})
	out := make(map[string]string, len(mi))
	for k, val := range mi {
		out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", val)
	}
	return out
}

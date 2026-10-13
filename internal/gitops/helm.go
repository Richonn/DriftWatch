package gitops

import (
	"fmt"
	"os"
	"strings"

	"github.com/Richonn/driftwatch/internal/cluster"
	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

type HelmOptions struct {
	ValuesFiles []string
	IncludeCRDs bool
}

func isHelmChart(dir string) bool {
	_, err := os.Stat(dir + "/Chart.yaml")
	return err == nil
}

func renderChart(chartDir string, opts HelmOptions) ([]cluster.Resource, error) {
	chrt, err := loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	warnMissingDeps(chrt)

	vals := chartutil.Values{}
	for k, v := range chrt.Values {
		vals[k] = v
	}
	for _, vf := range opts.ValuesFiles {
		extra, err := chartutil.ReadValuesFile(vf)
		if err != nil {
			return nil, fmt.Errorf("read values file %s: %w", vf, err)
		}
		vals = chartutil.CoalesceTables(extra, vals)
	}

	renderVals, err := chartutil.ToRenderValues(chrt, vals, chartutil.ReleaseOptions{
		Name:      chrt.Metadata.Name,
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		return nil, fmt.Errorf("prepare render values: %w", err)
	}

	rendered, err := engine.Render(chrt, renderVals)
	if err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	var resources []cluster.Resource
	for name, content := range rendered {
		if strings.HasSuffix(name, "/_helpers.tpl") || strings.HasSuffix(name, "/NOTES.txt") {
			continue
		}
		found, err := parseYAMLDocs(strings.NewReader(content), opts.IncludeCRDs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skipping rendered template %s: %v\n", name, err)
			continue
		}
		resources = append(resources, found...)
	}
	return resources, nil
}

func warnMissingDeps(chrt *helmchart.Chart) {
	if chrt.Metadata == nil || len(chrt.Metadata.Dependencies) == 0 {
		return
	}
	loaded := make(map[string]bool, len(chrt.Dependencies()))
	for _, dep := range chrt.Dependencies() {
		loaded[dep.Metadata.Name] = true
	}
	for _, dep := range chrt.Metadata.Dependencies {
		if !loaded[dep.Name] {
			fmt.Fprintf(os.Stderr, "warn: chart %q missing dependency %q — run `helm dependency update`\n",
				chrt.Metadata.Name, dep.Name)
		}
	}
}

package subcommands

import "testing"

func TestKubePrometheusStackValuesKeepMonitoringIntegration(t *testing.T) {
	values := getKubePrometheusStackVals(true, "http://mimir.example/mimir/api/v1/push")
	prometheus := values["prometheus"].(map[string]any)["prometheusSpec"].(map[string]any)
	if prometheus["serviceMonitorSelectorNilUsesHelmValues"] != false || prometheus["podMonitorSelectorNilUsesHelmValues"] != false {
		t.Fatal("Prometheus must discover ServiceMonitors and PodMonitors without Helm release labels")
	}
	if len(prometheus["remoteWrite"].([]any)) != 1 || len(prometheus["additionalScrapeConfigs"].([]any)) != 1 {
		t.Fatal("Prometheus must keep Mimir remote write and cgroups exporter scrape")
	}
	if values["kubeEtcd"].(map[string]any)["enabled"] != false || values["kubeProxy"].(map[string]any)["enabled"] != false {
		t.Fatal("unsupported control-plane scrapes must remain disabled")
	}
	if prometheus["nodeSelector"].(map[string]any)["monitoring"] != "true" {
		t.Fatal("Prometheus must use reserved monitoring node")
	}
}

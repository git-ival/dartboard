import { check, fail, sleep } from 'k6'
import { Trend, Gauge } from 'k6/metrics';
import http from 'k6/http'

// Resource count tracking metrics (using Gauges since they represent current state)
export const totalProjectsGauge = new Gauge('cluster_projects_total')
export const totalNamespacesGauge = new Gauge('cluster_namespaces_total')
export const totalPodsGauge = new Gauge('cluster_pods_total')
export const totalSecretsGauge = new Gauge('cluster_secrets_total')
export const totalConfigMapsGauge = new Gauge('cluster_configmaps_total')
export const totalServiceAccountsGauge = new Gauge('cluster_serviceaccounts_total')
export const totalClusterRolesGauge = new Gauge('cluster_clusterroles_total')
export const totalCRDsGauge = new Gauge('cluster_crds_total')

// API response time tracking metrics
export const systemImageAPITime = new Trend('api_systemimage_duration')
export const eventAPITime = new Trend('api_event_duration')
export const k8sEventAPITime = new Trend('api_k8sevent_duration')
export const settingsAPITime = new Trend('api_settings_duration')
export const clusterRoleAPITime = new Trend('api_clusterrole_duration')
export const crdAPITime = new Trend('api_crd_duration')
export const clusterRoleBindingAPITime = new Trend('api_clusterrolebinding_duration')
export const rkeAddonAPITime = new Trend('api_rkeaddon_duration')
export const configMapAPITime = new Trend('api_configmap_duration')
export const serviceAccountAPITime = new Trend('api_serviceaccount_duration')
export const secretAPITime = new Trend('api_secret_duration')
export const podAPITime = new Trend('api_pod_duration')
export const rkeServiceOptionAPITime = new Trend('api_rkeserviceoption_duration')
export const apiServiceAPITime = new Trend('api_apiservice_duration')
export const roleTemplateAPITime = new Trend('api_roletemplate_duration')
export const projectAPITime = new Trend('api_project_duration')
export const namespaceAPITime = new Trend('api_namespace_duration')

export const timingTag = { timing: "yes" }

export function getLocalClusterResourceCounts(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/counts`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'k8s/clusters/local/v1/counts can be queried': (r) => r.status === 200,
  })
  return JSON.parse(response.body)["data"][0]["counts"]
}

export function processResourceCounts(resourceCountObj) {
  let finalResourceCounts = {}

  if (resourceCountObj && typeof resourceCountObj === 'object') {
    Object.keys(resourceCountObj).forEach(resourceType => {
      let resourceData = resourceCountObj[resourceType]

      // Extract clean resource name from type (remove API version prefixes)
      let resourceName = resourceType.split('.').pop() || resourceType

      let resourceInfo = {}

      // Get total count from summary
      if (resourceData.summary && resourceData.summary.count !== undefined) {
        resourceInfo.totalCount = resourceData.summary.count
      }

      // Get namespaced counts if they exist
      if (resourceData.namespaces && typeof resourceData.namespaces === 'object') {
        resourceInfo.namespaces = {}
        Object.keys(resourceData.namespaces).forEach(namespace => {
          let namespaceData = resourceData.namespaces[namespace]
          if (namespaceData.count !== undefined) {
            resourceInfo.namespaces[namespace] = namespaceData.count
          }
        })
      }

      // Only add to final object if we have some data
      if (resourceInfo.totalCount !== undefined || resourceInfo.namespaces) {
        finalResourceCounts[resourceName] = resourceInfo
      }
    })
  }

  return finalResourceCounts
}

export function getLocalClusterSystemImageTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.rkek8ssystemimage`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.rkek8ssystemimage can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterEventTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/event`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/event can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterK8sEventTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/events.k8s.io.event`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/events.k8s.io.event can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterSettingsTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.setting`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.setting can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterClusterRoleTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/rbac.authorization.k8s.io.clusterrole`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/rbac.authorization.k8s.io.clusterrole can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterCRDTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/apiextensions.k8s.io.customresourcedefinition`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/apiextensions.k8s.io.customresourcedefinition can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterClusterRoleBindingTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/rbac.authorization.k8s.io.clusterrolebinding`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/rbac.authorization.k8s.io.clusterrolebinding can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterRKEAddonTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.rkeaddon`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.rkeaddon can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterConfigMapTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/configmap`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/configmap can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterServiceAccountTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/serviceaccount`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/serviceaccount can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterSecretTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/secret`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/secret can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterPodTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/pod`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/pod can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterRKEK8sServiceOptionTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.rkek8sserviceoption`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.rkek8sserviceoption can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterAPIServiceTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/apiregistration.k8s.io.apiservice`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/apiregistration.k8s.io.apiservice can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterRoleTemplateTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.roletemplate`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.roletemplate can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterProjectsTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/management.cattle.io.projects`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/management.cattle.io.projects can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function getLocalClusterNamespacesTiming(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/k8s/clusters/local/v1/namespaces`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
    tags: timingTag
  })
  check(response, {
    'k8s/clusters/local/v1/namespaces can be queried': (r) => r.status === 200,
  })
  return response.timings
}

export function processResourceTimings(baseUrl, cookies) {
  let timingsArr = {}

  // Get timing data for each resource type
  try {
    let systemImageTiming = getLocalClusterSystemImageTiming(baseUrl, cookies)
    if (systemImageTiming && systemImageTiming.duration) {
      timingsArr["management.cattle.io.rkek8ssystemimage"] = systemImageTiming.duration
    }

    let eventTiming = getLocalClusterEventTiming(baseUrl, cookies)
    if (eventTiming && eventTiming.duration) {
      timingsArr["event"] = eventTiming.duration
    }

    let k8sEventTiming = getLocalClusterK8sEventTiming(baseUrl, cookies)
    if (k8sEventTiming && k8sEventTiming.duration) {
      timingsArr["events.k8s.io.event"] = k8sEventTiming.duration
    }

    let settingsTiming = getLocalClusterSettingsTiming(baseUrl, cookies)
    if (settingsTiming && settingsTiming.duration) {
      timingsArr["management.cattle.io.setting"] = settingsTiming.duration
    }

    let clusterRoleTiming = getLocalClusterClusterRoleTiming(baseUrl, cookies)
    if (clusterRoleTiming && clusterRoleTiming.duration) {
      timingsArr["rbac.authorization.k8s.io.clusterrole"] = clusterRoleTiming.duration
    }

    let crdTiming = getLocalClusterCRDTiming(baseUrl, cookies)
    if (crdTiming && crdTiming.duration) {
      timingsArr["apiextensions.k8s.io.customresourcedefinition"] = crdTiming.duration
    }

    let clusterRoleBindingTiming = getLocalClusterClusterRoleBindingTiming(baseUrl, cookies)
    if (clusterRoleBindingTiming && clusterRoleBindingTiming.duration) {
      timingsArr["rbac.authorization.k8s.io.clusterrolebinding"] = clusterRoleBindingTiming.duration
    }

    let rkeAddonTiming = getLocalClusterRKEAddonTiming(baseUrl, cookies)
    if (rkeAddonTiming && rkeAddonTiming.duration) {
      timingsArr["management.cattle.io.rkeaddon"] = rkeAddonTiming.duration
    }

    let configMapTiming = getLocalClusterConfigMapTiming(baseUrl, cookies)
    if (configMapTiming && configMapTiming.duration) {
      timingsArr["configmap"] = configMapTiming.duration
    }

    let serviceAccountTiming = getLocalClusterServiceAccountTiming(baseUrl, cookies)
    if (serviceAccountTiming && serviceAccountTiming.duration) {
      timingsArr["serviceaccount"] = serviceAccountTiming.duration
    }

    let secretTiming = getLocalClusterSecretTiming(baseUrl, cookies)
    if (secretTiming && secretTiming.duration) {
      timingsArr["secret"] = secretTiming.duration
    }

    let podTiming = getLocalClusterPodTiming(baseUrl, cookies)
    if (podTiming && podTiming.duration) {
      timingsArr["pod"] = podTiming.duration
    }

    let rkeK8sServiceOptionTiming = getLocalClusterRKEK8sServiceOptionTiming(baseUrl, cookies)
    if (rkeK8sServiceOptionTiming && rkeK8sServiceOptionTiming.duration) {
      timingsArr["management.cattle.io.rkek8sserviceoption"] = rkeK8sServiceOptionTiming.duration
    }

    let apiServiceTiming = getLocalClusterAPIServiceTiming(baseUrl, cookies)
    if (apiServiceTiming && apiServiceTiming.duration) {
      timingsArr["apiregistration.k8s.io.apiservice"] = apiServiceTiming.duration
    }

    let roleTemplateTiming = getLocalClusterRoleTemplateTiming(baseUrl, cookies)
    if (roleTemplateTiming && roleTemplateTiming.duration) {
      timingsArr["management.cattle.io.roletemplate"] = roleTemplateTiming.duration
    }

    let projectsTiming = getLocalClusterProjectsTiming(baseUrl, cookies)
    if (projectsTiming && projectsTiming.duration) {
      timingsArr["management.cattle.io.project"] = projectsTiming.duration
    }

    let namespacesTiming = getLocalClusterNamespacesTiming(baseUrl, cookies)
    if (namespacesTiming && namespacesTiming.duration) {
      timingsArr["namespace"] = namespacesTiming.duration
    }

  } catch (error) {
    console.error('Error processing resource timings:', error)
  }

  return timingsArr
}


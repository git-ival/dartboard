import { check, fail, sleep } from 'k6'
import http from 'k6/http'

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

  } catch (error) {
    console.error('Error processing resource timings:', error)
  }

  return timingsArr
}


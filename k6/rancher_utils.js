import { check, fail, sleep } from 'k6'
import http from 'k6/http'

export function getCookies(baseUrl) {
  const response = http.get(`${baseUrl}/`)
  return http.cookieJar().cookiesForURL(response.url)
}

export function login(baseUrl, cookies, username, password) {
  const response = http.post(
    `${baseUrl}/v3-public/localProviders/local?action=login`,
    JSON.stringify({ "description": "UI session", "responseType": "cookie", "username": username, "password": password }),
    {
      headers: {
        accept: 'application/json',
        'content-type': 'application/json; charset=UTF-8',
      },
      cookies: { cookies },
    }
  )

  check(response, {
    'login works': (r) => r.status === 200 || r.status === 401,
  })

  return response.status === 200
}

export function timestamp() {
  return new Date().toISOString()
}

export function getUserId(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/v3/users?me=true`, {
    headers: {
      accept: 'application/json',
    },
    cookies: { cookies },
  })
  check(response, {
    'reading user details was successful': (r) => r.status === 200,
  })
  if (response.status !== 200) {
    fail('could not query user details')
  }

  return JSON.parse(response.body).data[0].id
}

export function getUserPreferences(baseUrl, cookies) {
  let response = http.get(`${baseUrl}/v1/userpreferences`, {
    headers: {
      accept: 'application/json',
    },
    cookies: { cookies },
  })
  check(response, {
    'preferences can be queried': (r) => r.status === 200,
  })
  return JSON.parse(response.body)["data"][0]
}

export function setUserPreferences(baseUrl, cookies, userId, userPreferences) {
  let response = http.put(
    `${baseUrl}/v1/userpreferences/${userId}`,
    JSON.stringify(userPreferences),
    {
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
      },
      cookies: { cookies },
    }
  )
  check(response, {
    'preferences can be set': (r) => r.status === 200,
  })
  return response;
}

export function getProjectById(baseUrl, cookies, projectId) {
  let res = http.get(`${baseUrl}/v3/projects?id=${projectId}`, { cookies: cookies })

  let checkOK = check(res, {
    '/v1/management.cattle.io.projects returns status 200': (r) => r.status === 200,
  })

  let projectData = JSON.parse(res.body)["data"]

  if (!checkOK || projectData.length !== 1) {
    console.log("\nProjectId: ", projectId, "\n")
    console.log("\nProjectData: ", projectData, "\n")
    fail("Status check failed or did not receive Project data")
  }

  return res
}

export function listProjects(baseUrl, cookies) {
  let res = http.get(`${baseUrl}/v1/management.cattle.io.projects`, { cookies: cookies })

  let checkOK = check(res, {
    '/v1/management.cattle.io.projects returns status 200': (r) => r.status === 200,
  })

  let projectsData = JSON.parse(res.body)["data"]

  if (!checkOK || projectsData === undefined || projectsData.length == 0) {
    fail("Status check failed or did not receive list of Projects data")
  }

  return res
}

export function getUserById(baseUrl, cookies, userId) {
  let res = http.get(`${baseUrl}/v3/users?id=${userId}`, { cookies: cookies })

  let checkOK = check(res, {
    '/v1/management.cattle.io.users returns status 200': (r) => r.status === 200 || r.status === 204,
  })

  let userData = JSON.parse(res.body)["data"]

  if (!checkOK || userData.length !== 1) {
    fail("Status check failed or did not receive User data")
  }

  return res
}

export function listUsers(baseUrl, cookies) {
  let res = http.get(`${baseUrl}/v3/users`, { cookies: cookies })

  let checkOK = check(res, {
    '/v1/management.cattle.io.users returns status 200': (r) => r.status === 200 || r.status === 204,
  })

  let usersData = JSON.parse(res.body)["data"]

  if (!checkOK || usersData === undefined || usersData.length == 0) {
    fail("Status check failed or did not receive list of Users data")
  }

  return res
}

export function listRoleTemplates(baseUrl, cookies) {
  let res = http.get(`${baseUrl}/v1/management.cattle.io.roletemplates`, { cookies: cookies })

  let checkOK = check(res, {
    '/v1/management.cattle.io.roletemplates returns status 200': (r) => r.status === 200 || r.status === 204,
  })

  let templatesData = JSON.parse(res.body)["data"]

  if (!checkOK || templatesData === undefined || templatesData.length == 0) {
    fail("Status check failed or did not receive list of RoleTemplates data")
  }

  return res
}


export function getGlobalRoles(baseUrl, cookies) {
  let res = http.get(`${baseUrl}/v1/management.cattle.io.globalroles`, { cookies: cookies })
  check(res, {
    '/v1/management.cattle.io.globalroles returns status 200': (r) => r.status === 200 || r.status === 204,
  })
  return res
}

export function firstLogin(baseUrl, cookies, bootstrapPassword, password) {
  let response

  if (login(baseUrl, cookies, "admin", bootstrapPassword)) {
    response = http.post(
      `${baseUrl}/v3/users?action=changepassword`,
      JSON.stringify({ "currentPassword": bootstrapPassword, "newPassword": password }),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json; charset=UTF-8',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'password can be changed': (r) => r.status === 200,
    })
    if (response.status !== 200) {
      fail('first password change was not successful')
    }
  }
  else {
    console.warn("bootstrap password already changed")
    if (!login(baseUrl, cookies, "admin", password)) {
      fail('neither bootstrap nor normal passwords were accepted')
    }
  }
  const userId = getUserId(baseUrl, cookies)
  const userPreferences = getUserPreferences(baseUrl, cookies);

  userPreferences["data"]["locale"] = "en-us"
  setUserPreferences(baseUrl, cookies, userId, userPreferences);

  response = http.get(
    `${baseUrl}/v1/management.cattle.io.settings`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: { cookies },
    }
  )
  check(response, {
    'Settings can be queried': (r) => r.status === 200,
  })
  const settings = JSON.parse(response.body)

  const firstLoginSetting = settings.data.filter(d => d.id === "first-login")[0]
  if (firstLoginSetting === undefined) {
    response = http.post(
      `${baseUrl}/v1/management.cattle.io.settings`,
      JSON.stringify({ "type": "management.cattle.io.setting", "metadata": { "name": "first-login" }, "value": "false" }),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'First login setting can be set': (r) => r.status === 201,
    })
  }
  else {
    firstLoginSetting["value"] = "false"
    response = http.put(
      `${baseUrl}/v1/management.cattle.io.settings/first-login`,
      JSON.stringify(firstLoginSetting),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'First login setting can be changed': (r) => r.status === 200,
    })
  }

  const eulaSetting = settings.data.filter(d => d.id === "eula-agreed")[0]
  if (eulaSetting === undefined) {
    response = http.post(
      `${baseUrl}/v1/management.cattle.io.settings`,
      JSON.stringify({ "type": "management.cattle.io.setting", "metadata": { "name": "eula-agreed" }, "value": timestamp(), "default": timestamp() }),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'EULA setting can be set': (r) => r.status === 201,
    })
  }
  else {
    eulaSetting["value"] = timestamp()
    response = http.put(
      `${baseUrl}/v1/management.cattle.io.settings/eula-agreed`,
      JSON.stringify(eulaSetting),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'EULA setting can be changed': (r) => r.status === 200,
    })
  }

  const telemetrySetting = settings.data.find(d => d.id === "telemetry-opt")
  if (telemetrySetting === undefined) {
    response = http.post(
      `${baseUrl}/v1/management.cattle.io.settings/telemetry-opt`,
      JSON.stringify({ "type": "management.cattle.io.setting", "metadata": { "name": "telemetry-opt", "value": "out" } }),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'telemetry setting can be set': (r) => r.status === 201,
    })
  }
  else {
    telemetrySetting["value"] = "out"
    response = http.put(
      `${baseUrl}/v1/management.cattle.io.settings/telemetry-opt`,
      JSON.stringify(telemetrySetting),
      {
        headers: {
          accept: 'application/json',
          'content-type': 'application/json',
        },
        cookies: { cookies },
      }
    )
    check(response, {
      'telemetry setting can be changed': (r) => r.status === 200,
    })
  }
}

export function createImportedCluster(baseUrl, cookies, name) {
  let response

  const userId = getUserId(baseUrl, cookies)
  const userPreferences = getUserPreferences(baseUrl, cookies);

  userPreferences["last-visited"] = "{\"name\":\"c-cluster-product\",\"params\":{\"cluster\":\"_\",\"product\":\"manager\"}}"
  userPreferences["locale"] = "en-us"
  userPreferences["seen-whatsnew"] = "\"v2.7.1\""
  userPreferences["seen-cluster"] = "_"
  setUserPreferences(baseUrl, cookies, userId, userPreferences)

  response = http.get(`${baseUrl}/v1/catalog.cattle.io.clusterrepos`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying clusterrepos works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v1/management.cattle.io.kontainerdrivers`, {
    headers: {
      accept: 'application/json',
      cookies: cookies,
    },
  })
  check(response, {
    'querying kontainerdrivers works': (r) => r.status === 200,
  })

  response = http.get(
    `${baseUrl}/v1/catalog.cattle.io.clusterrepos/rancher-charts?link=index`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(response, {
    'querying rancher-charts works': (r) => r.status === 200,
  })

  response = http.get(
    `${baseUrl}/v1/catalog.cattle.io.clusterrepos/rancher-partner-charts?link=index`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(response, {
    'querying rancher-partners-charts works': (r) => r.status === 200,
  })

  response = http.get(
    `${baseUrl}/v1/catalog.cattle.io.clusterrepos/rancher-rke2-charts?link=index`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(response, {
    'querying rancher-rke2-charts works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v3/clusterroletemplatebindings`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying clusterroletemplatebindings works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v1/management.cattle.io.roletemplates`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying roletemplates works': (r) => r.status === 200,
  })

  response = http.post(
    `${baseUrl}/v1/provisioning.cattle.io.clusters`,
    JSON.stringify({ "type": "provisioning.cattle.io.cluster", "metadata": { "namespace": "fleet-default", "name": name }, "spec": {} }),
    {
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
      },
      cookies: cookies,
    }
  )

  check(response, {
    'creating an imported cluster works': (r) => r.status === 201 || r.status === 409,
  })
  if (response.status === 409) {
    console.warn(`cluster ${name} already exists`)
  }

  response = http.get(
    `${baseUrl}/v1/provisioning.cattle.io.clusters/fleet-default/${name}`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(response, {
    'querying clusters works': (r) => r.status === 200,
  })
  if (!response.status === 200) {
    fail(`cluster ${name} not found`)
  }

  response = http.get(`${baseUrl}/v1/cluster.x-k8s.io.machinedeployments`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying machinedeployments works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v1/rke.cattle.io.etcdsnapshots`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying etcdsnapshots works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v1/management.cattle.io.nodetemplates`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying nodetemplates works': (r) => r.status === 200,
  })

  response = http.get(`${baseUrl}/v1/management.cattle.io.clustertemplates`, {
    headers: {
      accept: 'application/json',
    },
    cookies: cookies,
  })
  check(response, {
    'querying clustertemplates works': (r) => r.status === 200,
  })

  response = http.get(
    `${baseUrl}/v1/management.cattle.io.clustertemplaterevisions`,
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(response, {
    'querying clustertemplaterevisions works': (r) => r.status === 200,
  })
}

export function createProject(baseUrl, cookies, name, clusterId, userId) {
  let res = http.post(
    `${baseUrl}/v3/projects`,
    JSON.stringify({
      "type": "project",
      "name": name,
      "description": `Dartboard ${name}`,
      "annotations": {},
      "labels": {},
      "clusterId": clusterId,
      "creatorId": `local://${userId}`,
      "containerDefaultResourceLimit": {
        "limitsCpu": "4m",
        "limitsMemory": "5Mi",
        "requestsCpu": "2m",
        "limitsGpu": 6,
        "requestsMemory": "3Mi"
      },
      "resourceQuota": {
        "limit": {
          "configMaps": "9",
          "limitsMemory": "900Mi",
          "limitsCpu": "90m",
          "persistentVolumeClaims": "9000"
        }
      },
      "namespaceDefaultResourceQuota": {
        "limit": {
          "configMaps": "6",
          "limitsMemory": "600Mi",
          "limitsCpu": "60m",
          "persistentVolumeClaims": "6000"
        }
      }
    }),
    { cookies: cookies }
  )

  check(res, {
    '/v3/projects returns status 201': (r) => r.status === 201,
  })

  return res
}

/*
  Usernames are prefixed with "user", password defaults to "useruseruser" if not set
*/
export function createUser(baseUrl, cookies, displayName, userName, password = "useruseruser") {
  const res = http.post(`${baseUrl}/v3/users`,
    JSON.stringify({
      "type": "user",
      "name": displayName,
      "description": `Dartboard ${displayName}`,
      "enabled": true,
      "mustChangePassword": false,
      "password": password,
      "username": `user-${userName}`
    }),
    { cookies: cookies }
  )

  check(res, {
    '/v3/users returns status 201': (r) => r.status === 201,
  })

  return res
}

// Create a new RoleTemplate for the given context
/*
  Example:
  baseUrl = "myUrl"
  cookies = "my cookies"
  template = {
      "name": "child-role1",
      "apiGroups": [
        "management.cattle.io",
        "rbac.authorization.k8s.io"
      ],
      "resources": [
        "projects",
        "clusterroles"
      ],
      "verbs": [
      ["get", "list"],
      ["get", "list", "create"]
      ],
      "locked": false,
      "clusterCreatorDefault": false,
      "projectCreatorDefault": false,
      "context": "cluster" || "global" || "project",
      "roleTemplateIds": ["inheritedRoleTemplateIds"]
  }
  const res = createRoleTemplate(baseUrl, cookies, template)
*/
const defaultRoleTemplate = {
  "type": "roleTemplate",
  "name": "defaultDartboardRoleTemplate",
  "description": "Dartboard",
  "rules": [
    {
      "apiGroups": [
        "management.cattle.io"
      ],
      "resourceNames": [],
      "resources": [
        "projects"
      ],
      "verbs": [
        "get",
        "list"
      ]
    }
  ],
  "locked": false,
  "clusterCreatorDefault": false,
  "projectCreatorDefault": false,
  "context": "cluster",
  "roleTemplateIds": []
}
const inheritedRoleTemplate = {
  "name": "defaultInheritedRoleTemplate",
  "description": "Dartboard",
  "locked": false,
  "clusterCreatorDefault": false,
  "projectCreatorDefault": false,
  "context": "cluster",
  "roleTemplateIds": ["cluster-member"]
}
export function createRoleTemplate(baseUrl, cookies, template) {
  const res = http.post(`${baseUrl}/v3/roletemplates`,
    JSON.stringify(template),
    {
      headers: {
        accept: 'application/json',
      },
      cookies: cookies,
    }
  )
  check(res, {
    '/v3/roletemplate returns status 201': (r) => r.status === 201,
  })

  return res
}

export function createGlobalRoleBinding(baseUrl, params, userId, roles = ["user"]) {
  const res = http.post(
    `${baseUrl}/v3/globalrolebindings`,
    JSON.stringify({
      "type": "globalRoleBinding",
      "globalRoleId": roles,
      "userId": userId
    }),
    params
  )

  check(res, {
    '/v3/globalrolebindings returns status 201': (r) => r.status === 201 || r.status === 204,
  })

  return res;
}


export function createPRTB(baseUrl, cookies, projectId, roleTemplateId, userId) {
  const res = http.post(
    `${baseUrl}/v3/projectroletemplatebindings`,
    JSON.stringify({
      "type": "projectroletemplatebinding",
      "roleTemplateId": roleTemplateId,
      "userPrincipalId": `local://${userId}`,
      "projectId": projectId.replace("/", ":")
    }),
    {
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
      },
      cookies: cookies
    }
  );

  console.log("\nCreate PRTB Status: ", res.status, "\n")

  check(res, {
    '/v3/projectroletemplatebindings returns status 201 or 404': (r) => r.status === 201 || r.status === 404,
  })

  return res;
}

export function createCRTB(baseUrl, cookies, clusterId, roleTemplateId, userId) {
  const res = http.post(`${baseUrl}/v3/clusterroletemplatebindings`,
    JSON.stringify({
      "type": "clusterroletemplatebinding",
      "clusterId": clusterId,
      "roleTemplateId": roleTemplateId,
      "userPrincipalId": `local://${userId}`
    }),
    {
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
      },
      cookies: cookies
    }
  );

  console.log("\nCreate cRTB Status: ", res.status, "\n")

  check(res, {
    'POST v3/clusterroletemplatebindings returns status 201': (r) => r.status === 201,
  });

  return res;
}


export function logout(baseUrl, cookies) {
  const response = http.post(`${baseUrl}/v3/tokens?action=logout`, '{}', {
    headers: {
      accept: 'application/json',
      'content-type': 'application/json',
    },
    cookies: cookies
  })

  check(response, {
    'logging out works': (r) => r.status === 200,
  })
}

export function deleteProjectsByPrefix(baseUrl, cookies, prefix) {
  let deletedAll = true

  let res = listProjects(baseUrl, cookies)
  if (res.status !== 200) {
    console.log("list projects status: ", res.status)
    return false
  }

  JSON.parse(res.body)["data"].filter(r => ("description" in r["spec"]) && r["spec"]["description"].startsWith(prefix)).forEach(r => {
    res = http.del(`${baseUrl}/v3/projects/${r["id"].replace("/", ":")}`, { cookies: cookies })


    if (res.status !== 200 && res.status !== 204) {
      console.log("delete projects status: ", res.status)
      deletedAll = false
    }

    check(res, {
      'DELETE /v3/projects returns status 200': (r) => r.status === 200,
    })
  })

  return deletedAll
}

export function deleteUsersByPrefix(baseUrl, cookies, prefix) {
  let deletedAll = true

  let res = listUsers(baseUrl, cookies)
  if (res.status !== 200) {
    console.log("list users status: ", res.status)
    return false
  }

  JSON.parse(res.body)["data"].filter(r => ("description" in r) && r["description"].startsWith(prefix)).forEach(r => {
    res = http.del(`${baseUrl}/v3/users/${r["id"]}`, { cookies: cookies })

    if (res.status !== 200 && res.status !== 204) {
      console.log("delete user status: ", res.status)
      deletedAll = false
    }

    check(res, {
      'DELETE /v3/users returns status 200': (r) => r.status === 200 || r.status === 204,
    })
  })

  return deletedAll
}

export function deleteGlobalRolesByPrefix(baseUrl, cookies, prefix) {
  let deletedAll = true

  let res = getGlobalRoles(baseUrl, cookies)
  if (res.status !== 200) {
    return false
  }

  JSON.parse(res.body)["data"].filter(r => ("description" in r) && r["description"].startsWith(prefix)).forEach(r => {
    res = http.del(`${baseUrl}/v3/globalRoles/${r["id"]}`, { cookies: cookies })

    if (res.status !== 200) {
      deletedAll = false
    }

    check(res, {
      'DELETE /v3/globalRoles returns status 200': (r) => r.status === 200 || r.status === 204,
    })
  })

  return deletedAll
}

export function deleteRoleTemplatesByPrefix(baseUrl, cookies, prefix) {
  let deletedAll = true

  let res = listRoleTemplates(baseUrl, cookies)
  if (res.status !== 200) {
    console.log("list roletemplates status: ", res.status)
    return false
  }

  JSON.parse(res.body)["data"].filter(r => ("description" in r) && r["description"].startsWith(prefix)).forEach(r => {
    res = http.del(`${baseUrl}/v3/roletemplates/${r["id"].replace("/", ":")}`, { cookies: cookies })


    if (res.status !== 200) {
      console.log("delete roletemplates status: ", res.status)
      deletedAll = false
    }

    check(res, {
      'DELETE /v3/roletemplates returns status 200': (r) => r.status === 200,
    })
  })

  return deletedAll
}

// Retries result-returning function for up to 10 times
// until a non-409 status is returned, waiting for up to 1s
// between retries
export function retryOnConflict(f) {
  for (let i = 0; i < 9; i++) {
    const res = f()
    if (res.status !== 409) {
      return res
    }
    // expected conflict. Sleep a bit and retry
    sleep(Math.random())
  }
  // all previous attempts failed, try one last time
  return f()
}

// Check if version string A >= version string B.
// Does not account for non-standard semver, like "-rc" or "-alpha"
export function isVersionGEQ(versionA, versionB) {
  const partsA = versionA.split('.').map(x => parseInt(x, 10));
  const partsB = versionB.split('.').map(x => parseInt(x, 10));
  const maxLength = Math.max(partsA.length, partsB.length);

  for (let i = 0; i < 3; i++) {
    const partA = partsA[i] || 0;
    const partB = partsB[i] || 0;

    if (partA > partB) {
      return true;
    } else if (partA < partB) {
      return false;
    }
  }

  return true; // Versions are equal
}

// Check if version string A < version string B.
// Does not account for non-standard semver, like "-rc" or "-alpha"
export function isVersionLT(versionA, versionB) {
  const partsA = versionA.split('.').map(x => parseInt(x, 10));
  const partsB = versionB.split('.').map(x => parseInt(x, 10));
  const maxLength = Math.max(partsA.length, partsB.length);

  for (let i = 0; i < 3; i++) {
    const partA = partsA[i] || 0;
    const partB = partsB[i] || 0;

    if (partA < partB) {
      return true;
    } else if (partA > partB) {
      return false;
    }
  }

  return false; // Versions are equal
}

// Gets # of random elements from the given array and returns them as an array
export function getRandomElements(arr, numElements) {
  const shuffled = [...arr].sort(() => 0.5 - Math.random());
  return shuffled.slice(0, numElements);
}

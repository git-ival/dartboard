import { check, fail, sleep } from 'k6';
import exec from 'k6/execution';
import http from 'k6/http';
import { Gauge } from 'k6/metrics';
import {
  getCookies, login, logout, isVersionLT, deleteProjectsByPrefix,
  deleteUsersByPrefix, createProject, createUser,
  createRoleTemplate, listProjects, listUsers,
  createPRTB, createCRTB, getProjectById, getUserById,
  deleteRoleTemplatesByPrefix
} from "./rancher_utils.js";

// Parameters
const vus = 1
const projectCount = Number(__ENV.PROJECT_COUNT) || 10
const userCount = Number(__ENV.USER_COUNT) || 10
const customPRTBsPerUser = Number(__ENV.USER_COUNT) || 5
const customCRTBsPerUser = Number(__ENV.USER_COUNT) || 5
const enableCatalogRoles = isVersionLT(__ENV.RANCHER_VERSION, "2.11.0") || false

// Option setting
const baseUrl = __ENV.BASE_URL
const username = __ENV.USERNAME
const password = __ENV.PASSWORD

// Option setting
export const options = {
  insecureSkipTLSVerify: true,

  setupTimeout: '8h',

  scenarios: {
    createPRTBs: {
      executor: 'shared-iterations',
      exec: 'createPRTBs',
      vus: vus,
      iterations: projectCount,
      maxDuration: '1h',
    },
    createCRTBs: {
      executor: 'shared-iterations',
      exec: 'createCRTBs',
      vus: vus,
      iterations: userCount,
      maxDuration: '1h',
    }
  },
  thresholds: {
    checks: ['rate>0.99']
  }
}

// Custom metrics
const projectsMetric = new Gauge('test_projects')

// Test functions, in order of execution

export function setup() {
  // log in
  if (!login(baseUrl, {}, username, password)) {
    fail(`could not login to cluster`)
  }
  const cookies = getCookies(baseUrl)

  // delete leftovers, if any
  cleanup(cookies)

  let clusterIds = getClusterIds(cookies)
  let myId = getMyId(cookies)

  // Create Projects, and Users
  for (let numProjects = 1; numProjects <= projectCount; numProjects++) {
    let res = createProject(baseUrl, cookies, `Test Project ${numProjects}`, clusterIds[numProjects % clusterIds.length], myId)
    if (res.status !== 201) {
      console.log("create project status: ", res.status)
      fail("Failed to create all expected Projects")
    }
  }

  for (let numUsers = 1; numUsers <= userCount; numUsers++) {
    let res = createUser(baseUrl, cookies, `Test User ${numUsers}`, `${numUsers}`)
    if (res.status !== 201) {
      console.log("create user status: ", res.status)
      fail("Failed to create all expected Users")
    }
  }

  sleep(2)
  let projectsRes = listProjects(baseUrl, cookies)
  if (projectsRes.status !== 200) {
    fail("Failed to retrieve Projects")
  }
  let usersRes = listUsers(baseUrl, cookies)
  if (usersRes.status !== 200) {
    fail("Failed to retrieve Users")
  }

  let projects = JSON.parse(projectsRes.body)["data"].filter(p => ("displayName" in p["spec"]) && p["spec"]["displayName"].startsWith("Test "))
  let users = JSON.parse(usersRes.body)["data"].filter(p => ("displayName" in p) && p["displayName"].startsWith("Test "))

  // return data that remains constant throughout the test
  return {
    cookies: cookies,
    principalIds: getPrincipalIds(cookies),
    myId: myId,
    clusterIds: clusterIds,
    projects: projects,
    users: users,
  }
}

function getPrincipalIds(cookies) {
  const response = http.get(
    `${baseUrl}/v1/management.cattle.io.users`,
    { cookies: cookies }
  )
  if (response.status !== 200) {
    fail('could not list users')
  }
  const users = JSON.parse(response.body).data
  return users.filter(u => u["username"] != null).map(u => u["principalIds"][0])
}

function getMyId(cookies) {
  const response = http.get(
    `${baseUrl}/v3/users?me=true`,
    { cookies: cookies }
  )
  if (response.status !== 200) {
    fail('could not get my user')
  }
  return JSON.parse(response.body).data[0].principalIds[0]
}

function getClusterIds(cookies) {
  const response = http.get(
    `${baseUrl}/v1/management.cattle.io.clusters`,
    { cookies: cookies }
  )
  if (response.status !== 200) {
    fail('could not list clusters')
  }
  const clusters = JSON.parse(response.body).data
  return clusters.map(c => c["id"])
}

function cleanup(cookies) {
  let success = false
  let projectsDeleted = deleteProjectsByPrefix(baseUrl, cookies, "Dartboard ")
  let usersDeleted = deleteUsersByPrefix(baseUrl, cookies, "Dartboard ")
  let roleTemplatesDeleted = deleteRoleTemplatesByPrefix(baseUrl, cookies, "Dartboard ")
  console.log(projectsDeleted)
  console.log(usersDeleted)
  console.log(roleTemplatesDeleted)
  if (!projectsDeleted || !usersDeleted || !roleTemplatesDeleted) {
    fail("failed to delete all objects created by test")
  }
}

// TODO: On each iteration create 1 Project, select 1 cluster and:
/*
  Create Project Role
  Create CRTB
  Add each user to Project with PRTB
  Add each user to Cluster with CRTB
  Verify user has *ONLY* the expected permissions
*/

function createUserExpectFail(baseUrl, cookies, name, password = "useruseruser") {
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

  let checkOK = check(res, {
    '/v3/users returns status 401 or 403': (r) => r.status === 401 || r.status === 403,
  })

  let userData = JSON.parse(res.body).data

  if (!checkOK || userData.length > 0) {
    fail("Status check failed or received unexpected User data")
  }

  return res
}

function createProjectExpectFail(baseUrl, cookies, name, clusterId, userPrincipalId) {
  let res = http.post(
    `${baseUrl}/v3/projects`,
    JSON.stringify({
      "type": "project",
      "name": name,
      "description": `Dartboard ${name}`,
      "annotations": {},
      "labels": {},
      "clusterId": clusterId,
      "creatorId": `local://${userPrincipalId}`,
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

  let checkOK = check(res, {
    '/v3/projects returns status 401 or 403': (r) => r.status === 401 || r.status === 403,
  })

  let projectData = JSON.parse(res.body).data

  if (!checkOK || projectData.length > 0) {
    fail("Status check failed or received unexpected Project data")
  }

  return res
}

export function createPRTBs(data) {
  const i = exec.scenario.iterationInTest

  let projectRoleTemplate = {
    "type": "roleTemplate",
    "name": `Dartboard PRTB ${i}`,
    "description": `Dartboard Test Project RT ${i}`,
    "rules": [
      {
        "apiGroups": [
          "management.cattle.io"
        ],
        "resourceNames": [],
        "resources": [
          "project"
        ],
        "verbs": [
          "get",
          "list"
        ]
      }
    ],
    "external": false,
    "locked": false,
    "clusterCreatorDefault": false,
    "projectCreatorDefault": false,
    "context": "project",
    "roleTemplateIds": []
  }

  let res = createRoleTemplate(baseUrl, data.cookies, projectRoleTemplate)

  console.log("\nPRTB: ", JSON.parse(res.body), "\n")

  if (res.status !== 201) {
    fail("Could not create Project Role Template")
  }

  let roleTemplateId = JSON.parse(res.body).id

  let userIdx = i % data.users.length
  let user = getUserById(baseUrl, data.cookies, data.users[i].id)
  console.log("\nProject: ", data.projects[i], "\n")
  console.log("Index: ", i, "\n")
  console.log("\nUser: ", user, "\n")
  console.log("Index: ", i, "\n")

  res = createPRTB(baseUrl, data.cookies, data.projects[i].id, roleTemplateId, user.id)

  if (res.status !== 201) {
    console.log("\nResponse: ", JSON.stringify(res, null, 2), "\n")
    fail("Failed to create PRTB")
  }

  logout(baseUrl, data.cookies);
  sleep(1)

  // log in as user
  if (!login(baseUrl, {}, user.username, "useruseruser")) {
    fail(`could not login to cluster as ${username}`)
  }
  const cookies = getCookies(baseUrl)

  getProjectById(baseUrl, cookies, data.projects[i].id.replace("/", ":"))
  listProjects(baseUrl, cookies)
  createProjectExpectFail(baseUrl, cookies, `Test Create Project Should Fail ${i}`, data.clusterIds[i])
  logout(baseUrl, cookies);
}

export function createCRTBs(data) {
  const i = exec.scenario.iterationInTest

  let clusterRoleTemplate = {
    "type": "roleTemplate",
    "name": `Dartboard CRTB ${i}`,
    "description": `Dartboard Test Cluster RT ${i}`,
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
    "external": false,
    "locked": false,
    "clusterCreatorDefault": false,
    "projectCreatorDefault": false,
    "context": "cluster",
    "roleTemplateIds": []
  }

  let res = createRoleTemplate(baseUrl, data.cookies, clusterRoleTemplate)

  console.log("\nCRTB: ", JSON.parse(res.body), "\n")

  if (res.status !== 201) {
    fail("Could not create Project Role Template")
  }

  let roleTemplateId = JSON.parse(res.body).id

  let userIdx = i % data.users.length
  let clusterIdx = i % data.clusterIds.length
  let user = getUserById(baseUrl, data.cookies, data.users[i].id)
  console.log("\nClusterID: ", data.clusterIds[clusterIdx], "\n")
  console.log("Index: ", i, "\n")
  console.log("\nUser: ", user, "\n")
  console.log("Index: ", i, "\n")

  res = createCRTB(baseUrl, data.cookies, data.clusterIds[clusterIdx], roleTemplateId, data.users[i].id)

  if (res.status !== 201) {
    console.log("\nResponse: ", JSON.stringify(res, null, 2), "\n")
    fail("Failed to create CRTB")
  }

  logout(baseUrl, data.cookies);
  sleep(1)

  // log in as user
  if (!login(baseUrl, {}, user.username, "useruseruser")) {
    fail(`could not login to cluster as ${username}`)
  }
  const cookies = getCookies(baseUrl)

  // getUserById(baseUrl, data.cookies, data.users[userIdx].id)
  getProjectById(baseUrl, cookies, data.projects[i].id.replace("/", ":"))
  // listUsers(baseUrl, cookies)
  listProjects(baseUrl, cookies)
  // createUserExpectFail(baseUrl, cookies, `Test Create User Should Fail ${i}`)
  createProjectExpectFail(baseUrl, cookies, `Test Create Project Should Fail ${i}`, data.clusterIds[i])
  logout(baseUrl, cookies);
}

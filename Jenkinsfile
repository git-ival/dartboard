// Declarative Pipeline Syntax

import groovy.text.SimpleTemplateEngine
import com.cloudbees.groovy.cps.NonCPS

/**
 * Renders a GString‑style templateText, substituting in the binding map.
 * Must be @NonCPS because SimpleTemplateEngine and Template aren’t Serializable.
 * Note: This will fail if the template does not include one or more of the expected vars inside the bindings map
 * https://docs.groovy-lang.org/latest/html/api/groovy/text/SimpleTemplateEngine.html
 */
@NonCPS
String renderTemplateText(String templateText, Map binding) {
    def engine   = new SimpleTemplateEngine()
    def template = engine.createTemplate(templateText)
    return template.make(binding).toString()
}


pipeline {
    agent { label 'vsphere-vpn-1' }

    environment {
        // Define environment variables here.  These are available throughout the pipeline.
        imageName = 'dartboard'
        testsDir = './k6'
        envFile = ".env"
        qaseEnvFile = '.qase.env'
        k6EnvFile = 'k6.env'
        harvesterKubeconfig = 'harvester.kubeconfig'
        templateDartFile = 'template-dart.yaml'
        renderedDartFile = 'rendered-dart.yaml'
    }

    // No parameters block here—JJB YAML defines them

    stages {
        stage('Checkout') {
            steps {
              script {
                // Choose between the SCM config or an override from params.REPO
                def repoConfig = params.REPO ?
                  [[ url: params.REPO ]] :
                  scm.userRemoteConfigs
                // Choose between the default "main" branch or the override from params.BRANCH
                def branch = params.BRANCH ? params.BRANCH : "main"
                // Use `checkout scm` to checkout the repository
                checkout scm: [
                    $class: 'GitSCM',
                    branches: [[name: "*/${branch}"]],
                    userRemoteConfigs: repoConfig,
                    extensions: scm.extensions + [[$class: 'CleanCheckout']],
                ]
              }
            }
        }

        // TODO: Set up a QASE client to utilize these for logging test run results + artifacts
        stage('Create QASE Environment Variables') {
            steps {
                script {
                    def qase = 'REPORT_TO_QASE=' + params.REPORT_TO_QASE + '\n' +
                                'QASE_PROJECT_ID=' + params.QASE_PROJECT_ID + '\n' +
                                'QASE_RUN_ID=' + params.QASE_RUN_ID + '\n' +
                                'QASE_TEST_CASE_ID=' + params.QASE_TEST_CASE_ID + '\n' +
                                'QASE_AUTOMATION_TOKEN=' + credentials('QASE_AUTOMATION_TOKEN') + '\n' // Use credentials plugin
                    writeFile file: qaseEnvFile, text: qase
                }
            }
        }

        stage('Configure and Build') {
            steps {
              script {
                sh 'printenv'
                echo "Storing env in file"
                sh "printenv > ${env.envFile}"
                sh "cat ${env.envFile}"

                echo "PRE-EXISTING IMAGES:"
                sh "docker image ls"

                // This will run `docker build -t my-image:main .`
                docker.build("${env.imageName}:latest")

                echo "NEW IMAGES:"
                sh "docker image ls"
                sh 'ls -al'
              }
            }
        }

        stage('Prepare Parameter Files') {
          steps {
            script {
              // Write out each multi-line string parameter into its own file
              writeFile file: env.k6EnvFile,           text: params.K6_ENV
              writeFile file: env.harvesterKubeconfig, text: params.HARVESTER_KUBECONFIG
              writeFile file: env.templateDartFile,    text: params.DART_FILE

              // Dump to verify
              echo "---- k6.env ----"
              sh "cat ${env.k6EnvFile}"
              echo "---- harvester.kubeconfig ----"
              sh "cat ${env.harvesterKubeconfig}"
              echo "---- template-dart.yaml ----"
              sh "cat ${env.templateDartFile}"
            }
          }
        }

        stage('Setup SSH Keys') {
          steps {
            script {
              echo 'PRE-SHELL WORKSPACE:'
              sh 'ls -al'
              // Decode the base64‐encoded private key into a file named after SSH_KEY_NAME
              // Write the public key string into a .pub file
              sh "echo ${env.SSH_PEM_KEY} | base64 -di > ${env.SSH_KEY_NAME}.pem"
              sh "chmod 600 ${env.SSH_KEY_NAME}.pem"

              sh "echo ${env.SSH_PUB_KEY} > ${env.SSH_KEY_NAME}.pub"
              sh "chmod 644 ${env.SSH_KEY_NAME}.pub"
              echo "PUB KEY:"
              sh "cat ${env.SSH_KEY_NAME}.pub"
            }
          }
        }

        stage('Render Dart file') {
          steps {
            script {
              // 1) Read the raw template file into a String
              def rawTemplate = readFile file: env.templateDartFile  // readFile step reads workspace files

              // 2) Build a binding map of all the env vars to be substituted
              def binding = [
                HARVESTER_KUBECONFIG: env.harvesterKubeconfig,
                SSH_KEY_NAME        : env.SSH_KEY_NAME,
              ]

              // 3) Call the helper render method
              echo "RENDERING TEMPLATE:"
              def rendered = renderTemplateText(rawTemplate, binding)

              // 4) Write the fully‐rendered YAML to file
              writeFile file: env.renderedDartFile, text: rendered

              // sh "envsubst < ${env.DART_FILE} > rendered-dart.yaml"
              echo "RENDERED DART:"
              sh "cat ${env.renderedDartFile}"
            }
          }
        }

        stage('Setup Infrastructure') {
            agent {
              docker {
                image "${env.imageName}:latest"
                reuseNode true
                args "--entrypoint='' --env-file ${env.envFile}"
              }
            }
            steps {
              script {
                echo 'WORKSPACE:'
                sh 'ls -al'
                sh "dartboard --dart ${env.renderedDartFile} deploy"
              }
            }
        }

        stage('Run Validation Tests') {
          agent {
              docker {
                image "${env.imageName}:latest"
                reuseNode true
                args "--entrypoint='' --env-file ${envFile}"
              }
            }
            steps {
              script {
                // if the user uploaded a K6_ENV file, source it so all its KEY=VALUE lines
                // become environment variables for the k6 process
                // `set` docs: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html

                // Compute the output filename in Groovy
                def baseName = params.K6_TEST.replaceFirst(/\.js$/, '')
                def outJson  = "${baseName}-output.json"

                if (fileExists(env.K6_ENV_FILE) && params.K6_ENV?.trim()) {
                  sh """
                    set -o allexport
                    source ${env.K6_ENV_FILE}
                    set +o allexport
                    k6 run --out json="${outJson}" ${env.TESTS_DIR}/${params.K6_TEST}
                  """
                } else {
                  sh "k6 run --out json=\"${outJson}\" ${env.TESTS_DIR}/${params.K6_TEST}"
                }
              }
            }
        }

        /*
        Because all docker stages share the same container and workspace (due to `reuseNode true`),
        any files written in the container (e.g. terraform.tfstate, terraform.tfstate.backup, or k6 output.json)
        end up directly on the Jenkins agent’s workspace.
        */
        stage('Archive Artifacts') {
            steps {
                echo "Archiving Terraform state and K6 test results..."
                // wildcard for any *.tfstate or backup, plus our k6 json output
                archiveArtifacts artifacts: '**/*.tfstate*, **/*.output.json', fingerprint: true
            }
        }
    }

    post {
      always {
        script {
            sh "docker image rm ${env.imageName}"
            echo "POST-CLEANUP IMAGES:"
            sh "docker image ls"
        }
      }
    }
}

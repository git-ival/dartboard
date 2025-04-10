// Declarative Pipeline Syntax

pipeline {
    agent { label 'vsphere-vpn-1' }

    environment {
        // Define environment variables here.  These are available throughout the pipeline.
        imageName = 'dartboard'
        testsDir = './k6'
        envFile = ".env"
        qaseEnvFile = '.qase.env'

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
                    sh '''
                      echo "WORKSPACE:"
                      ls -al
                      echo "PRE-EXISTING IMAGES:"
                      docker image ls
                    '''
                    // This will run `docker build -t my-image:main .`
                    docker.build("${env.imageName}:latest")
                    sh '''
                      echo "NEW IMAGES:"
                      docker image ls
                    '''
                }
            }
        }

        stage('Setup Infrastructure') {
            agent {
              docker {
                image "${env.imageName}:latest"
                label 'vsphere-vpn-1'
                reuseNode true
              }
            }
            steps {
              script {
                // Decode the base64‐encoded private key into a file named after SSH_KEY_NAME
                // Write the public key string into a .pub file
                sh '''
                  echo "${SSH_PEM_KEY}" | base64 -d > ${SSH_KEY_NAME}
                  chmod 600 ${SSH_KEY_NAME}.pem

                  echo "${SSH_PUB_KEY}" > ${SSH_KEY_NAME}.pub
                  chmod 644 ${SSH_KEY_NAME}.pub
                '''

                // Render the DART_FILE using `envsubst`, substituting in HARVESTER_KUBECONFIG and other vars
                // docs: https://man7.org/linux/man-pages/man1/envsubst.1.html
                sh '''
                  envsubst < "${params.DART_FILE}" > rendered-dart.yaml
                  dartboard --dart rendered-dart.yaml deploy
                '''
              }
            }
        }

        stage('Run Validation Tests') {
          agent {
              docker {
                image "${env.imageName}:latest"
                label 'vsphere-vpn-1'
                reuseNode true
              }
            }
            steps {
                script {
                  // if the user uploaded a K6_ENV file, source it so all its KEY=VALUE lines
                  // become environment variables for the k6 process
                  // `set` docs: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html
                  if (params.K6_ENV) {
                    sh '''
                      set -o allexport
                      source "${params.K6_ENV}"
                      set +o allexport
                      k6 run --out json="${params.K6_TEST%.js*}-output.json" ${testsDir}/"${params.K6_TEST}"
                    '''
                  } else {
                    // no env‐file, just run k6 and use any defaults provided in the script itself
                    sh '''
                      k6 run --out json="${params.K6_TEST%.js*}-output.json" ${testsDir}/"${params.K6_TEST}"
                    '''
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
          sh '''
            docker image rm ${env.imageName}
            echo "POST-CLEANUP IMAGES:"
            docker image ls
          '''
        }
      }
    }
}

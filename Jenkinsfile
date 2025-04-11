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

        stage('Setup Infrastructure') {
            agent {
              docker {
                image "${env.imageName}:latest"
                reuseNode true
                args "--entrypoint='' --env-file ${envFile}"
              }
            }
            steps {
              script {
                echo 'PRE-SHELL WORKSPACE:'
                sh 'ls -al'
                // Decode the base64‐encoded private key into a file named after SSH_KEY_NAME
                // Write the public key string into a .pub file
                sh "cat ${env.SSH_PEM_KEY} | base64 -d > ${env.SSH_KEY_NAME}"
                sh "chmod 600 ${env.SSH_KEY_NAME}.pem"

                sh "cat ${env.SSH_PUB_KEY} > ${env.SSH_KEY_NAME}.pub"
                sh "chmod 644 ${env.SSH_KEY_NAME}.pub"
                echo "PUB KEY:"
                cat "${env.SSH_KEY_NAME}.pub"
                sh "envsubst < ${env.DART_FILE} > rendered-dart.yaml"
                echo "RENDERED DART:"
                sh "cat rendered-dart.yaml"
                sh 'dartboard --dart rendered-dart.yaml deploy'
                // sh '''
                //   echo "\${SSH_PEM_KEY}" | base64 -d > \${env.SSH_KEY_NAME}
                //   chmod 600 \${env.SSH_KEY_NAME}.pem

                //   echo "\${SSH_PUB_KEY}" > \${env.SSH_KEY_NAME}.pub
                //   chmod 644 \${env.SSH_KEY_NAME}.pub
                //   echo "PUB KEY:"
                //   cat \${env.SSH_KEY_NAME}.pub

                //   envsubst < "\${params.DART_FILE}" > rendered-dart.yaml
                //   echo "RENDERED DART:"
                //   cat rendered-dart.yaml
                //   dartboard --dart rendered-dart.yaml deploy
                // '''
                echo 'WORKSPACE:'
                sh 'ls -al'
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
                if (params.K6_ENV) {
                  sh '''
                    set -o allexport
                    source "${params.K6_ENV}"
                    set +o allexport
                    k6 run --out json="${params.K6_TEST%.js*}-output.json" ${testsDir}/"${params.K6_TEST}"
                  '''
                } else {
                  // no env‐file, just run k6 and use any defaults provided in the script itself
                    sh "k6 run --out json=\"\${K6_TEST%.js*}-output.json\" ${env.testsDir}${params.K6_TEST}"
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

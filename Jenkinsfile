pipeline {
    agent {
        dockerfile {
            label 'master'
            args '-v /jenkins_home/tools:/jenkins_home/tools'
        }
    }
    environment {
        scannerHome = tool 'SonarScanner'
    }
    stages {
        stage('Clone') {
            steps {
                checkout scm
            }
        }
        stage('Tools') {
            steps {
                ansiColor('xterm') {
                    sh 'make tools'
                }
            }
        }
        stage('Dependencies') {
            steps {
                ansiColor('xterm') {
                    sh 'make dependencies'
                }
            }
        }
        stage('Static Checks') {
            steps {
                ansiColor('xterm') {
                    sh 'make check'
                }
            }
        }
        stage('Tests') {
            steps {
                ansiColor('xterm') {
                    sh 'make test'
                }
            }
        }
        stage('SonarQube PR analysis') {
            steps {
                configFileProvider([
                    configFile(fileId: 'sonarqube-ssl', variable: 'SONARQUBE_CERT'),
                ]) {
                    sh '''
                        mkdir -p ssl
                        chmod a+rwx ssl
                        if [ ! -e ssl/sonarcacert.ks ];then
                            keytool -import -file ${SONARQUBE_CERT} -keystore ssl/sonarcacert.ks -storepass changeit -noprompt
                        fi
                    '''
                }
                script {
                    pullId = "-Dsonar.pullrequest.key=${env.ghprbPullId}"
                    sourcheBranch = "-Dsonar.pullrequest.branch=${env.ghprbSourceBranch}"
                    targetBranch = "-Dsonar.pullrequest.base=${env.ghprbTargetBranch}"
                    env.SONAR_SCANNER_OPTS = "-Djavax.net.ssl.trustStore=ssl/sonarcacert.ks -Djavax.net.ssl.trustStorePassword=changeit"
                }
                withSonarQubeEnv('SonarQube') {
                    sh "${scannerHome}/bin/sonar-scanner ${pullId} ${sourcheBranch} ${targetBranch}"
                }
            }
        }
    }
    post {
        always {
            junit "**/junit.xml"
            step([
                $class: 'CoberturaPublisher',
                autoUpdateHealth: false,
                autoUpdateStability: false,
                coberturaReportFile: 'coverage/coverage.xml',
                failUnhealthy: true,
                failUnstable: true,
                maxNumberOfBuilds: 0,
                onlyStable: false,
                sourceEncoding: 'ASCII',
                zoomCoverageChart: false,
                lineCoverageTargets: '100.0, 90.0, 50.0',
            ])
        }
    }
}

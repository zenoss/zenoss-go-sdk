#! groovy

MAKE='make -f Makefile'

node('docker') {
    stage('Checkout') {
        checkout scm
    }

    currentBuild.displayName = "PR #${env.ghprbPullId}@${env.NODE_NAME}"
    configFileProvider([
        configFile(fileId: 'global', variable: 'GLOBAL'),
    ]) {
        global = load env.GLOBAL
    }

    withEnv([
        "COMMIT_SHA=${env.ghprbActualCommit}",
        "IMAGE_TAG=${env.ghprbActualCommit.substring(0,8)}",
        "PROJECT_NAME=notification-${env.BUILD_NUMBER}",
        "ROOTDIR=${global.HOST_WORKSPACE}"
        ]) {
        try {
            stage('Unit tests') {
                ansiColor('xterm') {
                    sh("mkdir ./coverage/")
                    sh("chmod -R o+w .")
                    sh("${MAKE} test-containerized")
                }
            }

            stage('SonarQube PR analysis') {
                String scannerHome = tool 'SonarScanner'
                List<String> args = [
                    "${scannerHome}/bin/sonar-scanner",
                    '-Dproject.settings=./sonar-project.properties',
                    "-Dsonar.pullrequest.key=${env.ghprbPullId}",
                    "-Dsonar.pullrequest.branch=${env.ghprbSourceBranch}",
                    "-Dsonar.pullrequest.base=${env.ghprbTargetBranch}",
                    '-Dsonar.buildbreaker.skip=true',
                ]
                withSonarQubeEnv('sonarqube.zenoss.io') {
                    sh args.join(' ')
                }
            }

            stage('Validate test results') {
                junit 'junit.xml'
                recordCoverage(
                    tools: [[parser: 'COBERTURA', pattern: 'coverage/coverage.xml']],
                    qualityGates: [[threshold: 90.0, metric: 'LINE', baseline: 'PROJECT', criticality: 'FAILURE']]
                )
            }
        } finally {
            stage('Clean test environment') {
                ansiColor('xterm') {
                    sh("${MAKE} clean")
                }
            }
        }
    }
}

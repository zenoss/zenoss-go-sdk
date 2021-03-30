pipeline {
    agent { dockerfile { label '!qa-integrations' } }
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

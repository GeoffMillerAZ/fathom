// vars/buildApp.groovy: a shared-library global variable.
def call(Map config = [:]) {
    pipeline {
        agent any
        stages {
            stage('Build') {
                steps {
                    sh "make ${config.target}"
                    helperStep()
                }
            }
        }
    }
}

def helperStep() {
    echo 'helping'
}

String version() { return '1.0' }

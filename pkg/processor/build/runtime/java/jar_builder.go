package java

import (
	"text/template"
	"path"
	"os"

	"github.com/nuclio/nuclio-sdk"
)

var gradleTemplateCode = `
plugins {
  id 'com.github.johnrengelman.shadow' version '2.0.2'
  id 'java'
}

repositories {
    mavenCentral()
}

dependencies {
	{{ range .Dependencies }}
	compile group: '{{.Group}}', name: '{{.Name}}', version: '{{.Version}}'
	{{ end }}

    compile files('./nuclio-sdk-1.0-SNAPSHOT.jar')
}

jar {
  manifest {
    attributes(
      'Main-Class': '{{.Handler}}'
    )
  }
}

// Output jar in this directory
tasks.withType(Jar) {
    destinationDir = file("$rootDir")
}

shadowJar {
   baseName = 'handler'
   classifier = null  // Don't append "all" to jar name
}
`

// Dependency is a Java dependency
type Dependency struct {
	Group string
	Name string
	Version string
}

// JarBuilder build handler jar from sources
type JarBuilder struct {
	Depedencies []*Dependency
	Handler string
	
	workDir string
	logger nuclio.Logger
}

// NewJarBuilder returns a new Java builder
func NewJarBuilder(logger nuclio.Logger) *JarBuilder {
	return &JarBuilder{
		logger: logger,
	}
}

func (jb *JarBuilder) createGradleFile() error {
	gradleTemplate, err := template.New("gradle").Parse(gradleTemplateCode)
	if err != nil {
		return err
	}

	gradleFile, err := os.Create(path.Join(jb.workDir, "build.gradle"))
	if err != nil {
		return err
	}

	defer gradleFile.Close()
	return gradleTemplate.Execute(gradleFile, jb)
}

// TODO: Build using docker

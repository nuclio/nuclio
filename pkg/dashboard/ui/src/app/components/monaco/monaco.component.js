(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclMonaco', {
            templateUrl: 'monaco/monaco.tpl.html',
            controller: NclMonacoController
        });

    function NclMonacoController($scope) {
        var ctrl = this;

        $scope.themes = ['vs-dark', 'vs', 'hc-black'];
        $scope.selectedTheme = $scope.themes[0];

        $scope.codeFiles = [
            {
                'filename': 'sample.go',
                'language': 'go',
                'code': `// You can edit this code!
// Click here and start typing.
package main

import "fmt"

func main() {
\tfmt.Println("Hello, 世界")
}`,
            },
            {
                'filename': 'sample-file.js',
                'language': 'javascript',
                'code': ['function x() {',
                    '\tconsole.log("Hello world - this is monaco!");',
                    '}'].join('\n'),
            },
            {
                'filename': 'sample-file.cs',
                'language': 'csharp',
                'useSpaces': false,
                'code': `using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace VS
{
\tclass Program
\t{
\t\tstatic void Main(string[] args)
\t\t{
\t\t\tProcessStartInfo si = new ProcessStartInfo();
\t\t\tfloat load= 3.2e02f;

\t\t\tsi.FileName = @"tools\\\\node.exe";
\t\t\tsi.Arguments = "tools\\\\simpleserver.js";

\t\t\tProcess.Start(si);
\t\t}
\t}
}`,
            },
            {
                'filename': 'sample.py',
                'language': 'python',
                'code': `def hello():
\tprint("Hello World") 
\treturn `,
            }
        ];
        $scope.selectedCodeFile = $scope.codeFiles[0];

        $scope.useSpaces = false;
    }
}());

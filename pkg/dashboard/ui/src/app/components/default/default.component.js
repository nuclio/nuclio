(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzDefault', {
            templateUrl: 'default/default.tpl.html',
            controller: DefaultController
        });

    function DefaultController($scope) {
        var ctrl = this;

        // $scope.filename = 'sample-file.js';
        // $scope.code = ['function x() {',
        // '\tconsole.log("Hello world - this is monaco!");',
        // '}'].join('\n');
        // $scope.language = 'javascript';

        $scope.filename = 'sample-file.cs';
        $scope.code = `using System;
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
}`
        $scope.language = 'csharp';
        $scope.languages = ['go','csharp', 'javascript', 'html', 'python', 'cpp'];
    }
}());

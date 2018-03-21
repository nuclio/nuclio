(function () {
    'use strict';

    angular.module('iguazio.app')
        .directive('igzMonacoEditor', function ($interval) {
            console.log('in igzMonacoEditor');
            function link(scope, element, attrs) {
                var editorElement = element[0];
                require(['vs/editor/editor.main'], function () {
                    var editor = window.monaco.editor.create(editorElement, {
                        value: scope.code,
                        language: scope.language
                    });
                    // TODO - look up api docs to find a suitable event to handle as the onDidChangeModelContent event only seems to fire for certain changes!
                    // As a fallback, currently updating scope on a timer...
                    // editor.onDidChangeModelContent = function(e){
                    //   console.log('modelContent changed');
                    //   scope.code = editor.getValue();
                    //   scope.$apply();
                    // }

                    $interval(function updateScope() {
                        scope.code = editor.getValue();
                    }, 1000);
                });
            }

            return {
                link: link,
                scope: {
                    code: '=code',
                    language: '=language'
                }
            };
        });
}());

(function () {
    'use strict';

    angular.module('iguazio.app')
        .directive('igzMonacoEditor', function ($interval) {
            console.log('in igzMonacoEditor');
            function link(scope, element, attrs) {
                var editorElement = element[0];
                require(['vs/editor/editor.main'], function () {
                    var useSpaces, editorTheme;
                    if (angular.isUndefined(scope.useSpaces) || scope.useSpaces === null) {
                        useSpaces = true; // TODO - decide on default value when not bound
                    } else {
                        useSpaces = scope.useSpaces;
                    }
                    if (angular.isUndefined(scope.editorTheme) || scope.editorTheme === null) {
                        editorTheme = 'vs'; // TODO - decide on default value when not bound
                    } else {
                        editorTheme = scope.editorTheme;
                    }

                    var editor = window.monaco.editor.create(editorElement, {
                        language: scope.language,
                        theme: editorTheme
                    });
                    // TODO - look up api docs to find a suitable event to handle as the onDidChangeModelContent event only seems to fire for certain changes!
                    // As a fallback, currently updating scope on a timer...
                    // editor.onDidChangeModelContent = function(e){
                    //   console.log('modelContent changed');
                    //   scope.code = editor.getValue();
                    //   scope.$apply();
                    // }


                    editor
                        .getModel()
                        .updateOptions({ insertSpaces: useSpaces });
                    editor.setValue(scope.code);

                    $interval(function updateScope() {
                        scope.code = editor.getValue();
                        window.monaco.editor.setModelLanguage(editor.getModel(), scope.language)
                    }, 1000);
                });
            }

            return {
                link: link,
                scope: {
                    code: '=code',
                    language: '=language',
                    useSpaces: '=useSpaces',
                    editorTheme: '=editorTheme'
                }
            };
        });
}());

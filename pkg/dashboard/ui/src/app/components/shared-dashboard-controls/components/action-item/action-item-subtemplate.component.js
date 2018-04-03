(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionItemSubtemplate', {
            bindings: {
                action: '<'
            },
            template: '<div class="igz-action-item-subtemplate"></div>',
            controller: IgzActionItemSubtemplateController
        });

    function IgzActionItemSubtemplateController($compile, $element) {
        var ctrl = this;

        ctrl.newScope = null;

        ctrl.$postLink = postLink;

        //
        // Hook methods
        //

        /**
         * Post linking method
         */
        function postLink() {
            var subTemplate = angular.element(ctrl.action.template);
            $element.find('.igz-action-item-subtemplate').append(subTemplate);

            ctrl.newScope = ctrl.action.scope.$new();
            ctrl.newScope.action = ctrl.action;
            $compile(subTemplate)(ctrl.newScope);

            ctrl.action.destroyNewScope = destroyNewScope;
        }

        //
        // Private method
        //

        /**
         * Destroy new created scope. Scope needs to be removed to prevent errors when viewing tags on the browse page.
         * And it needs to be done when updating panel actions
         */
        function destroyNewScope() {
            ctrl.newScope.$destroy();
        }
    }
}());


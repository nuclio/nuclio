(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('ProjectsService', ProjectsService);

    function ProjectsService($i18next, i18next) {
        return {
            initProjectActions: initProjectActions
        };

        //
        // Public methods
        //

        /**
         * Project actions
         * @returns {Object[]} - array of actions
         */
        function initProjectActions() {
            var lng = i18next.language;

            return [
                {
                    label: $i18next.t('common:DELETE', {lng: lng}),
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    confirm: {
                        message: $i18next.t('functions:DELETE_PROJECTS_CONFIRM', {lng: lng}),
                        description: $i18next.t('functions:DELETE_PROJECT_DESCRIPTION', {lng: lng}),
                        yesLabel: $i18next.t('common:YES_DELETE', {lng: lng}),
                        noLabel: $i18next.t('common:CANCEL', {lng: lng}),
                        type: 'nuclio_alert'
                    }
                },
                {
                    label: $i18next.t('common:EDIT', {lng: lng}),
                    id: 'edit',
                    icon: 'igz-icon-edit',
                    active: true
                },
                {
                    label: $i18next.t('common:EXPORT', {lng: lng}),
                    id: 'export',
                    icon: 'igz-icon-export-yml',
                    active: true
                }
            ];
        }
    }
}());

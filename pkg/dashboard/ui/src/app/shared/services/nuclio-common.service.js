(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioCommonService', NuclioCommonService);

    function NuclioCommonService($q, NuclioFunctionsDataService, lodash) {
        return {
            duplicateFunction: duplicateFunction
        };

        /**
         * Duplicates selected function with a new name
         * @param {Object} version
         * @param {string} funcName
         * @param {string} projectID
         * @returns {Promise}
         */
        function duplicateFunction(version, funcName, projectID) {
            var duplicateFunctionDeferred = $q.defer();
            var newFunction = lodash.pick(version, 'spec');

            lodash.set(newFunction, 'metadata.name', funcName);

            NuclioFunctionsDataService.getFunctions(projectID)
                .then(function (response) {
                    if (lodash.isEmpty(lodash.filter(response, ['metadata.name', funcName]))) {
                        NuclioFunctionsDataService.createFunction(newFunction, projectID)
                            .then(function () {
                                duplicateFunctionDeferred.resolve();
                            });
                    } else {
                        duplicateFunctionDeferred.reject();
                    }
                });

            return duplicateFunctionDeferred.promise;
        }
    }
}());

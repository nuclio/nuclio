(function () {
    'use strict';

    /* global moment:false */
    /* global _:false */

    /**
     * All third-party components' global variables placed here as constants
     * in order to inject them in other angular app components
     */
    angular.module('nuclio.app')
        .constant('moment', window.moment)
        .constant('lodash', window._);
}());

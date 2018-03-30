(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('NavigationTabsService', NavigationTabsService);

    function NavigationTabsService(lodash, ConfigService) {
        return {
            getNavigationTabsConfig: getNavigationTabsConfig
        };

        //
        // Public methods
        //

        /**
         * Returns navigation tabs config depending on current state
         * @param {string} state
         * @returns {Array}
         */
        function getNavigationTabsConfig(state) {
            var navigationTabsConfigs = {
                'app.container': getContainersConfig(),
                'app.cluster': getClustersConfig(),
                'app.events': getEventsConfig(),
                'app.storage-pool': getStoragePoolsConfig(),
                'app.identity': getIdentityConfig(),
                'app.control-panel': getControlPanelConfig()
            };
            var stateTest = state.match(/^[^.]*.[^.]*/);

            return lodash.get(navigationTabsConfigs, stateTest[0], []);
        }

        //
        // Private methods
        //

        /**
         * Returns containers navigation tabs config
         * @returns {Array.<Object>}
         */
        function getContainersConfig() {
            var config = [
                {
                    tabName: 'Overview',
                    uiRoute: 'app.container.overview',
                    capability: 'containers.overview'
                },
                {
                    tabName: 'Browse',
                    uiRoute: 'app.container.browser',
                    capability: 'containers.browse'
                },
                {
                    tabName: 'Data Access Policy',
                    uiRoute: 'app.container.data-access-policy',
                    capability: 'containers.dataPolicy'
                }
            ];

            if (ConfigService.isStagingMode()) {
                config.push(
                    {
                        tabName: 'Data Lifecycle',
                        uiRoute: 'app.container.data-lifecycle',
                        capability: 'containers.dataLifecycle'
                    }
                );
            }

            if (ConfigService.isDemoMode()) {
                config.splice(1, 0,
                    {
                        tabName: 'Analytics',
                        uiRoute: 'app.container.analytics',
                        capability: 'containers.analytics'
                    }
                );
            }

            return config;
        }

        /**
         * Returns clusters navigation tabs config
         * @returns {Array.<Object>}
         */
        function getClustersConfig() {
            return [
                {
                    tabName: 'Nodes',
                    uiRoute: 'app.cluster.nodes',
                    capability: 'clusters.nodes'
                }
            ];
        }

        /**
         * Returns storage pools navigation tabs config
         * @returns {Array.<Object>}
         */
        function getStoragePoolsConfig() {
            var config = [
                {
                    tabName: 'Overview',
                    uiRoute: 'app.storage-pool.overview',
                    capability: 'storagePools.overview'
                },
                {
                    tabName: 'Devices',
                    uiRoute: 'app.storage-pool.devices',
                    capability: 'storagePools.listDevices'
                }
            ];

            if (ConfigService.isStagingMode()) {
                config.splice(1, 0,
                    {
                        tabName: 'Containers',
                        uiRoute: 'app.storage-pool.containers',
                        capability: 'storagePools.listContainers'
                    }
                );
            }

            return config;
        }

        /**
         * Returns control panel navigation tabs config
         * @returns {Array.<Object>}
         */
        function getControlPanelConfig() {
            return [{
                tabName: 'Logs',
                uiRoute: 'app.control-panel.logs'
            }];
        }

        /**
         * Returns identity navigation tabs config
         * @returns {Array.<Object>}
         */
        function getIdentityConfig() {
            var config = [
                {
                    tabName: 'Users',
                    uiRoute: 'app.identity.users',
                    capability: 'identity.users'
                },
                {
                    tabName: 'Groups',
                    uiRoute: 'app.identity.groups',
                    capability: 'identity.groups'
                }
            ];

            if (ConfigService.isStagingMode()) {
                config.push({
                    tabName: 'IDP',
                    uiRoute: 'app.identity.idp',
                    capability: 'identity.idp'
                });
            }

            return config;
        }

        /**
         * Returns events navigation tabs config
         * @returns {Array.<Object>}
         */
        function getEventsConfig() {
            var config = [
                {
                    tabName: 'Event Log',
                    uiRoute: 'app.events.event-log',
                    capability: 'events.eventLog'
                },
                {
                    tabName: 'Alerts',
                    uiRoute: 'app.events.alerts',
                    capability: 'events.alerts'
                }
            ];

            if (ConfigService.isStagingMode()) {
                config.push(
                    {
                        tabName: 'Escalation',
                        uiRoute: 'app.events.escalation',
                        capability: 'events.escalations'
                    },
                    {
                        tabName: 'Tasks',
                        uiRoute: 'app.events.tasks',
                        capability: 'events.tasks'
                    }
                );
            }

            return config;
        }
    }
}());

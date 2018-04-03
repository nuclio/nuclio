(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('FunctionsService', FunctionsService);

    function FunctionsService() {
        return {
            getClassesList: getClassesList,
            initVersionActions: initVersionActions
        };

        //
        // Public methods
        //

        /**
         * Returns classes list by type
         * @returns {Object[]} - array of classes
         */
        function getClassesList(type) {
            var classesList = {
                trigger: [
                    {
                        id: 'kafka',
                        name: 'Kafka',
                        attributes: [
                            {
                                name: 'topic',
                                pattern: 'string'
                            },
                            {
                                name: 'partitions',
                                pattern: 'array'
                            }
                        ]
                    },
                    {
                        id: 'rabbit_mq',
                        name: 'RabbitMQ',
                        attributes: [
                            {
                                name: 'exchangeName',
                                pattern: 'string'
                            },
                            {
                                name: 'queueName',
                                pattern: 'string'
                            },
                            {
                                name: 'topic',
                                pattern: 'array'
                            }
                        ]
                    },
                    {
                        id: 'nats',
                        name: 'Nats',
                        attributes: [
                            {
                                name: 'topic',
                                pattern: 'string'
                            }
                        ]
                    },
                    {
                        id: 'cron',
                        name: 'Cron',
                        attributes: [
                            {
                                name: 'event',
                                type: 'object',
                                attributes: [
                                    {
                                        name: 'body',
                                        type: 'string'
                                    },
                                    {
                                        name: 'headers',
                                        type: 'map'
                                    }
                                ]
                            }
                        ]
                    },
                    {
                        id: 'eventhub',
                        name: 'Eventhub',
                        attributes: [
                            {
                                name: 'sharedAccessKeyName',
                                pattern: 'string'
                            },
                            {
                                name: 'sharedAccessKeyValue',
                                pattern: 'string'
                            },
                            {
                                name: 'namespace',
                                pattern: 'string'
                            },
                            {
                                name: 'eventHubName',
                                pattern: 'string'
                            },
                            {
                                name: 'consumerGroup',
                                pattern: 'string'
                            },
                            {
                                name: 'partitions',
                                pattern: 'array'
                            }
                        ]
                    },
                    {
                        id: 'http',
                        name: 'HTTP',
                        attributes: [
                            {
                                name: 'ingresses',
                                type: 'map',
                                attributes: [
                                    {
                                        name: 'host',
                                        type: 'string'
                                    },
                                    {
                                        name: 'paths',
                                        type: 'array'
                                    }
                                ]
                            }
                        ]
                    },
                    {
                        id: 'kinesis',
                        name: 'Kinesis',
                        attributes: [
                            {
                                name: 'accessKeyID',
                                pattern: 'string'
                            },
                            {
                                name: 'secretAccessKey',
                                pattern: 'string'
                            },
                            {
                                name: 'regionName',
                                pattern: 'string'
                            },
                            {
                                name: 'streamName',
                                pattern: 'string'
                            },
                            {
                                name: 'shards',
                                pattern: 'string'
                            }
                        ]
                    }
                ],
                binding: [
                    {
                        id: 'kafka',
                        name: 'Kafka',
                        attributes: [
                            {
                                name: 'topic',
                                pattern: 'string'
                            },
                            {
                                name: 'partitions',
                                pattern: 'array'
                            }
                        ]
                    },
                    {
                        id: 'azure',
                        name: 'Azure Event hub',
                        attributes: [
                            {
                                name: 'secret',
                                pattern: 'string'
                            }
                        ]
                    }
                ]
            };

            return classesList[type];
        }

        /**
         * Actions for Action panel
         * @returns {Object[]} - array of actions
         */
        function initVersionActions() {
            var actions = [
                {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-edit',
                    active: true,
                    capability: 'nuclio.functions.versions.edit'
                },
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.functions.versions.delete',
                    confirm: {
                        message: 'Are you sure you want to delete selected version?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'critical_alert'
                    }
                }
            ];

            return actions;
        }
    }
}());

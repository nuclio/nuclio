module.exports = function () {
    var jstoxml = require('jstoxml').toXML;
    var fs = require('fs');
    var pathModule = require('path');
    var currentDate = new Date();
    var foldersArray = require('./mock-data.repository/folders.json');

    /**
     * Returns formatted mock
     * @param incomingData
     * @param {Object} element
     * @returns {Object} formatted mock data
     */
    this.generateXml = function (incomingData, element, params) {
        return new GenerateXml().generate(incomingData, element, params);
    };

    /**
     * Returns generated mock of Headers data
     * @param {Object} propertyToOverwrite
     */
    this.generateHeaders = function (propertyToOverwrite) {
        return new HeadersData().generate(propertyToOverwrite);
    };

    /**
     * Returns session,
     * @param {object} sessionData
     * @returns {Object} NewSession
     */
    this.generateSession = function (sessionData, updatedUser) {
        return new NewSession(sessionData, updatedUser)
    };

    this.generateResourcesList = function (request) {
        return resourcesList(request);
    };

    /**
     * Returns container by given container id (request.param), field (request.query.field),
     * include list - parsed to Array (request.query.include)
     * @param {string} params_id
     * @param {Object} query_fields = query.field
     * @param {Array} query_include = parsed to Array query.include
     * @returns {Object}
     */
    this.generateContainer = function (params_id, query_fields, query_include) {
        return new ContainerData().generate(params_id, query_fields, query_include);
    };

    /**
     * Returns container new container,
     * @param {object} data
     * @returns {Object} NewContainer
     */
    this.generateNewContainer = function (data) {
        return new NewContainer(data)
    };

    /**
     * Returns container new network,
     * @param {object} data
     * @returns {Object} NewNetwork
     */
    this.generateNewNetwork = function (data) {
        return new NewNetwork(data)
    };

    /**
     * Returns container new escalation,
     * @param {object} data
     * @returns {Object} NewEscalation
     */
    this.generateNewEscalation = function (data) {
        return new NewEscalation(data)
    };

    /**
     * Returns container new user,
     * @param {Object} data
     * @returns {Object} NewUser
     */
    this.generateNewUser = function (data) {
        return new NewUser(data)
    };

    /**
     * Returns new tenant,
     * @param {object} data
     * @returns {Object} NewTenant
     */
    this.generateNewTenant = function (data) {
        return new NewTenant(data)
    };

    /**
     * Returns Password Reset,
     * @param {Object} data
     * @returns {Object} PasswordReset
     */
    this.generatePasswordReset = function (data) {
        return new PasswordReset(data)
    };

    /**
     * Returns container new user group,
     * @param {object} data
     * @returns {Object} NewUserGroup
     */
    this.generateUserGroup = function (data) {
        return new NewUserGroup(data)
    };

    /**
     * Returns new storage pool,
     * @param {object} data
     * @returns {Object} NewStoragePool
     */
    this.generateNewStoragePool = function (data) {
        return new NewStoragePool(data)
    };

    /**
     * Return mock that contains json object by name
     * @param {string} type - name of json
     * @returns {{data}}
     */
    this.getMockData = function (type) {
        return {
            "data": getEntitiesArray(type)
        };
    };

    /**
     * Copy of getResource method from mockBackEnd/app.js
     * @param {Object} params
     * @param {Object|undefined} query
     * @param {string} type
     * @param {?Object} filter
     * @returns {Object}
     */
    this.getResourceMock = function (params, query, type, filter) {
        return getResource(params, query, type, filter);
    };

    /**
     * Returns generated mock of overview statistic
     * @param {Object} requestParams = {containerId: string, projection: string, from: string, pointsInSelectionLimit: string, until: string}
     * @param {Object} statistic
     * @returns {Object} formatted mock data
     */
    this.generateOverviewStatistic = function (requestParams, statistic) {
        return new OverviewStatistic().generate(requestParams, statistic);
    };

    /**
     * Returns generated mock of overview sessions
     * @param {string} interfaceId
     * @param {Array} inclusionList
     */
    this.generateOverviewSessions = function (interfaceId, inclusionList) {
        return new OverviewSessions().generate(interfaceId, inclusionList);
    };

    this.updateBackRelationships = function (updated, outdated) {
        return updateBackRelations(updated, outdated);
    };

    this.getResourceCollection = function (type) {
        return getEntities(type);
    };

    this.setResourceCollection = function (collection) {
        _.forEach(users, function(value, key) {
            if (value !== collection[key]) {
                value.relationships = _.cloneDeep(collection[key].relationships);
            }
        });
    };

    var getEntitiesArray = function (type) {
        return require('./mock-data.repository/' + type + '.json');
    };

    var getEntities = function (type) {
        return _.keyBy(getEntitiesArray(type), 'id');
    };

    var artifactBundles = {};
    var passwordResets = {};

    var containers = (function () {
        return getEntities('containers');
    })();

    var interfaces = (function () {
        return getEntities('interfaces');
    })();

    var storagePools = (function () {
        return getEntities('storage-pools');
    })();

    var workloadProfiles = (function () {
        return getEntities('workload-profiles');
    })();

    var users = (function () {
        return getEntities('users');
    })();

    var policies = (function () {
        return getEntities('policies');
    })();

    var userGroups = (function () {
        return getEntities('user-groups');
    })();

    var networks = (function () {
        return getEntities('networks');
    })();

    var jobs = (function () {
        return getEntities('jobs');
    })();

    var alerts = (function () {
        return getEntities('alerts');
    })();

    var escalationPolicies = (function () {
        return getEntities('escalation-policies');
    })();

    var escalationPolicyFilters = (function () {
        return getEntities('escalation-policy-filters');
    })();

    var escalationPolicyRules = (function () {
        return getEntities('escalation-policy-rules');
    })();

    var events = (function () {
        return getEntities('events');
    })();

    var dataPolicyGroups = (function () {
        return getEntities('data-policy-groups');
    })();

    var dataPolicyLayers = (function () {
        return getEntities('data-policy-layers');
    })();

    var dataPolicyRules = (function () {
        return getEntities('data-policy-rules');
    })();

    var dataPolicyDatetimeRanges = (function () {
        return getEntities('data-policy-datetime-ranges');
    })();

    var objectCategories = (function () {
        return getEntities('object-categories');
    })();

    var dataPolicyFunctions = (function () {
        return getEntities('data-policy-functions');
    })();

    var dataLifecycleLayers = (function () {
        return getEntities('data-lifecycle-layers');
    })();

    var dataLifecycleRules = (function () {
        return getEntities('data-lifecycle-rules');
    })();

    var interfaceDetails = (function () {
        return getEntities('interfaces-details');
    })();

    var browserTags = (function () {
        return getEntities('browser-tags');
    })();

    var storageDevices = (function () {
        return getEntities('storage-devices');
    })();

    var storagePoolDevices = (function () {
        return getEntities('storage-pool-devices');
    })();

    var clusters = (function () {
        return getEntities('clusters');
    })();

    var clusterNodes = (function () {
        return getEntities('nodes');
    })();

    var tenants = (function () {
        return getEntities('tenants');
    })();

    var storageClasses = (function () {
        return getEntities('storage-classes');
    })();

    var collections = {
        'artifact_bundle': artifactBundles,
        'job': jobs,
        'data_policy_datetime_range': dataPolicyDatetimeRanges,
        'data_policy_function': dataPolicyFunctions,
        'data_policy_group': dataPolicyGroups,
        'data_policy_layer': dataPolicyLayers,
        'data_policy_rule': dataPolicyRules,
        'browser_tag': browserTags,
        'storage_pool_device': storagePoolDevices,
        'storage_device': storageDevices,
        'container': containers,
        'data_lifecycle_layer': dataLifecycleLayers,
        'data_lifecycle_rule': dataLifecycleRules,
        'interface': interfaces,
        'interface_details': interfaceDetails,
        'network': networks,
        'object_category': objectCategories,
        'storage_pool': storagePools,
        'storage_class': storageClasses,
        'user': users,
        'user_group': userGroups,
        'policy': policies,
        'workload_profile': workloadProfiles,
        'cluster': clusters,
        'node': clusterNodes,
        'tenant': tenants,
        // events
        'alert': alerts,
        'escalation_policy': escalationPolicies,
        'escalation_policy_filter': escalationPolicyFilters,
        'escalation_policy_rule': escalationPolicyRules,
        'event': events,
        'password_reset': passwordResets
    };

    var biDirectionalRelationships = {
        'container': [
            {
                own:     { name: 'interfaces', asList: true  },
                related: { name: 'container',  asList: false }
            }
        ],
        'interface': [
            {
                own:     { name: 'container',  asList: false },
                related: { name: 'interfaces', asList: true  }
            }
        ],
        'session': [
            {
                own:     { name: 'user',     asList: false },
                related: { name: 'sessions', asList: true  }
            }
        ],
        'user': [
            {
                own:     { name: 'user_groups', asList: true },
                related: { name: 'users',       asList: true }
            },
            {
                own:     { name: 'sessions', asList: true  },
                related: { name: 'user',     asList: false }
            }
        ],
        'user_group': [
            {
                own:     { name: 'users',       asList: true },
                related: { name: 'user_groups', asList: true }
            },
            {
                own:     { name: 'policies', asList: true },
                related: { name: 'groups',   asList: true }
            }
        ],
        'escalation_policy_filter': [
            {
                own:     { name: 'policies',  asList: true },
                related: { name: 'filters',   asList: true }
            }
        ],
        'escalation_policy_rule': [
            {
                own:     { name: 'policy', asList: false },
                related: { name: 'rules',  asList: true  }
            }
        ],
        'data_policy_layer': [
            {
                own:     { name: 'container',          asList: false },
                related: { name: 'data_policy_layers', asList: true  }
            },
            {
                own:     { name: 'groups', asList: true  },
                related: { name: 'layer',  asList: false }
            },
            {
                own:     { name: 'rules', asList: true  },
                related: { name: 'layer', asList: false }
            }
        ],
        'data_policy_group': [
            {
                own:     { name: 'layer',  asList: false },
                related: { name: 'groups', asList: true  }
            },
            {
                own:     { name: 'rules', asList: true  },
                related: { name: 'group', asList: false }
            }
        ],
        'data_policy_rule': [
            {
                own:     { name: 'layer', asList: false },
                related: { name: 'rules', asList: true  }
            },
            {
                own:     { name: 'group', asList: false },
                related: { name: 'rules', asList: true  }
            }
        ],
        'data_lifecycle_layer': [
            {
                own:     { name: 'container',             asList: false },
                related: { name: 'data_lifecycle_layers', asList: true  }
            },
            {
                own:     { name: 'rules', asList: true  },
                related: { name: 'layer', asList: false }
            }
        ],
        'data_lifecycle_rule': [
            {
                own:     { name: 'layer', asList: false },
                related: { name: 'rules', asList: true  }
            }
        ]
    };

    // ----------------------------------------------------------

    /**
     * Creates an object that helps to generate S3Mocks
     * @constructor
     */
    function GenerateXml() {
        /**
         * Returns formatted mock
         * @param incomingData
         * @param {Object} element
         * @returns {Object} formatted mock
         */
        this.generate = function (incomingData, element, params) {
            return formatPreparedData(prepareForFormatting(incomingData, element, params));
        };

        /**
         * Format data to XML
         * @param {Object} preparedData
         * @returns {string}
         */
        function formatPreparedData(preparedData) {
            var xmlPrefix = '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>';

            var deafaultDataXML = jstoxml(preparedData, {header: false});

            return xmlPrefix + deafaultDataXML;
        }

        /**
         * Returns object that contains data prepared for formatting
         * @param {Object} incomingData - mock object that has to be prepared to formatting into XML
         * @param {?Object} element
         * @param {?Object} params
         * @returns {{defaultData: (defaultJson.response.data),
             * contentArray: Array, commonPrefixArray: Array}}
         */
        function prepareForFormatting(incomingData, element, params) {
            params = _.defaultTo(params, {});
            var data = [];

            if (element && element.remove) {
                _.pullAt(incomingData, element.remove);
            }
            var defaultObject = {
                _name: 'ListBucketResult',
                _attrs: {
                    xmlns: 'http://s3.amazonaws.com/doc/2006-03-01/'
                },
                _content: [
                    {Name: 'iguazio-test-bucket'},
                    {Prefix: params['prefix'] || ''},
                    {Marker: params['marker'] || ''},
                    {MaxKeys: params['max-keys'] || 1000},
                    {Delimiter: '/'},
                    {IsTruncated: params['IsTruncated'] || false}
                ]
            };

            if (_.has(params, 'NextMarker')) {
                defaultObject._content.push({"NextMarker": params['NextMarker']})
            }

            if (params['prefix-only']) {
                _.forEach(incomingData, function (element) {
                    if (element.CommonPrefixes) {
                        data.push({"CommonPrefixes":{"Prefix": _.get(element, ['CommonPrefixes', 'Prefix'])}})
                    }
                })
            } else {
                setDefaultContentsProperties(incomingData);
                data.push(incomingData);
            }

            defaultObject._content.push(data);

            return defaultObject;
        }

        /**
         * Check whether the Contents field in incoming data Array has missed properties, if "Yes" set default values on their place
         * @param {Array} data - Array of incoming data objects
         */
        function setDefaultContentsProperties(data) {

            // check whether the incoming data is not empty
            if (data && data.length > 0) {
                // go via all objects in incoming data
                for (var i = 0; i < data.length; i++) {
                    fillMissedContentsFields(data[i]);
                }
            }
        }

        /**
         * Check whether the Contents field in incoming data object has missed properties end fill them
         * @param {Object} data - incoming data object
         */
        function fillMissedContentsFields(data) {
            var defaultContents = {
                Key: "someStubFile.txt",
                LastModified: "2015-07-15T06:38:32.603Z",
                ETag: "\"59eaa4dbd96af86372c450388f3ce0de\"",
                Size: "100",
                Owner: {
                    ID: "some_user_id",
                    DisplayName: "Omri"
                },
                StorageClass: "STANDARD"
            };

            // check whether the incoming data object has property "Contents"
            if (data.Contents != null) {
                // if True - go via all properties in default contents obj
                for (var property in defaultContents) {
                    // check whether the incoming data object.Contents has missed properties
                    if (data.Contents[property] == null) {
                        // fill missed property from default contents obj into incoming data.Contents
                        data.Contents[property] = defaultContents[property];
                    }
                }
            }
        }
    }

    /**
     * Interprets a parameter of Graphite (value and unit) and returns the number of seconds it represents
     * @param param the parameter to interpret, e.g. "-4h" for four hours or "-5d" for five days
     * @param {number} [offset=0] number of seconds to shift the value evaluated from `param`
     * @returns {number} the number of seconds represented by the given parameter (non-negative)
     */
    function convertToSeconds(param, offset) {
        if (moment(param, 'HH:mm_YYYYMMDD', true).isValid()) {
            return Number(moment(param, 'HH:mm_YYYYMMDD').valueOf()) / 1000;
        }

        // parse param
        var matches = param.match(/^(\-)?(\d+)(s|min|h|d|w|mon|y)$/);

        // if param is in unsupported format - return NaN
        if (_.isNull(matches)) {
            return NaN;
        }

        // get the sign
        var sign = matches[1] === '-' ? -1 : 1;

        // get the numeric value
        var value = Number(matches[2]);

        // get the time unit
        var unit = matches[3];

        // holds the number of seconds implied by each time unit
        var factor = 1;

        // set factor's value according to time unit
        switch (unit) {
            case "min":
                factor *= 60;
                break;
            case "h":
                factor *= 60 * 60;
                break;
            case "d":
                factor *= 60 * 60 * 24;
                break;
            case "w":
                factor *= 60 * 60 * 24 * 7;
                break;
            case "mon":
                factor *= 60 * 60 * 24 * 30;
                break;
            case "y":
                factor *= 60 * 60 * 24 * 365;
                break;
        }

        return (_.isNil(offset) ? 0 : offset) + sign * factor * value;
    }

    /**
     * Creates an object that helps to generate Overview statistic mocks
     * @constructor
     */
    function OverviewStatistic() {
        var cacheStoragePoolId = '0';

        /**
         * Generate overview statistic
         * @param {Object} requestParams = {containerId: string, projection: string, from: string, pointsInSelectionLimit: string, until: string}
         * @param {Object} statistic
         * @returns {{data}}
         */
        this.generate = function (requestParams, statistic) {
            var request = {
                query: {},
                params: {}
            };
            var statisticForResponse;

            // property for generating the fake statistic for given fake container
            request.container = requestParams.container;

            // prepare request queries
            request.query.filter = requestParams.filter;
            request.query.url = requestParams.url;
            request.query.from = requestParams.from;
            request.query.until = requestParams.until;
            request.query.interval = requestParams.interval;
            request.query.projection = requestParams.projection;
            switch (requestParams.refer) {
                case 'containers':
                    // prepare params
                    request.params.cid = requestParams.container ? requestParams.container.id : requestParams.containerId;
                    request.params.secondary = requestParams.secondary;
                    request.params.sid = requestParams.sid;
                    statisticForResponse = getContainerStatistics(request, statistic);
                    break;
                case 'storage_pools':
                    // prepare params
                    request.params.id = requestParams.container ? requestParams.container.id : requestParams.containerId;
                    statisticForResponse = getStoragePoolsStatistics(request, statistic);
                    break;
                case 'interfaces':
                    // prepare params
                    request.params.id = requestParams.container ? requestParams.container.id : requestParams.containerId;
                    statisticForResponse = getInterfacesStatistics(request, statistic);
                    break;
                case 'clusters':
                    // prepare params
                    request.params.cid = requestParams.container ? requestParams.container.id : requestParams.containerId;
                    request.params.nid = requestParams.nid;
                    statisticForResponse = getClustersStatistics(request, statistic);
                    break;
                case 'nodes':
                    // prepare params
                    request.params.nid = requestParams.container ? requestParams.container.id : requestParams.containerId;
                    statisticForResponse = getNodesStatistics(request, statistic);
                    break;
                case 'events':
                    statisticForResponse = getEventsStatistics(request, statistic);
                    break;
            }

            return statisticForResponse;
        };

        var getContainerStatistics = function (request, statistic) {
            var containerId = request.params.cid;
            var containerList = _.isNumber(containerId) ? // if primary resource's id is specified
                _.compact([containers[containerId]])    : // then generate statistics for this resource only
                _.values(containers);                     // otherwise generate statistics for the entire collection

            var ids = ['container.' + containerId];
            var secondaryTypes = ['storage_pools', 'interfaces'];

            // for each container, for each of its related secondary resource (storage pool or interface) - make an id
            _.forEach(containerList, function (container) {
                var secondaryResourceList = _.chain(container.relationships)
                    .pick(_.defaultTo(request.params.secondary, secondaryTypes)) // {name1: {data: [{id1, type1}]}, name2: {data: [{id2, type2}]}}
                    .values()    // [{data: [{id1, type1}]}, {data: [{id2, type2}]}]
                    .map('data') // [[{id1, type1}], [{id2, type2}]]
                    .flatten()   // [{id1, type1}, {id2, type2}]
                    .value();

                // unless only interfaces were requested - add virtual "cache" storage pool
                if (request.params.secondary !== 'interfaces') {
                    secondaryResourceList.push({type: 'storage_pool', id: cacheStoragePoolId});
                }

                // if an id for secondary resource is provided - filter to include only it
                if (_.isString(request.params.sid)) {
                    secondaryResourceList = _.filter(secondaryResourceList, 'id', request.params.sid)
                }

                _.forEach(secondaryResourceList, function (secondaryResource) {
                    ids.push(['container', container.id, secondaryResource.type, secondaryResource.id].join('.'));
                });
            });

            return getStatistics(request, statistic, ids);
        };

        var getStoragePoolsStatistics = function (request, statistic) {
            var storagePoolId = request.params.id;
            var storagePoolList = _.isString(storagePoolId) ? // if primary resource's id is specified
                _.compact([storagePools[storagePoolId]]) : // then generate statistics for this resource only
                _.values(storagePools);                       // otherwise generate statistics for the entire collection

            var ids = ['storage_pool.' + storagePoolId];

            // for each storage pool, for each of its related devices - make an id
            _.forEach(storagePoolList, function (storagePool) {
                if (!_.includes(request.query.url, 'container')) {
                    var devicesIdList = _.map(storagePool.relationships.storage_pool_devices.data, 'id');

                    _.forEach(devicesIdList, function (deviceId) {
                        ids.push(['storage_pool', storagePool.id, 'storage_pool_device', deviceId].join('.'));
                    });
                }

                if (!_.includes(request.query.url, 'storage_pool_devices')) {
                    var containerIdList = _.map(storagePool.relationships.containers.data, 'id');

                    _.forEach(containerIdList, function (containerId) {
                        ids.push(['container', containerId, 'storage_pool', storagePool.id].join('.'));
                    });
                }
            });

            return getStatistics(request, statistic, ids);
        };

        var getInterfacesStatistics = function (request, statistic) {
            var interfaceId = request.params.id || '';
            var interfaceResource;

            if (request.container) {
                interfaceResource = _.compact([request.container]);
            } else {
                interfaceResource = interfaces[interfaceId];
            }

            var ids = [];

            // for each of the interface's related containers - make an id
            var containerId = interfaceResource.relationships.container.data.id;

            if (_.isObject(interfaceResource)) {
                ids.push(['container', containerId, 'interface', interfaceId].join('.'));
            }

            return getStatistics(request, statistic, ids);
        };

        var getClustersStatistics = function (request, statistic) {
            var clusterId = request.params.cid;
            var clusterList = _.isString(clusterId) ? // if primary resource's id is specified
                _.compact([clusters[clusterId]])    : // then generate statistics for this resource only
                _.values(clusters);                   // otherwise generate statistics for the entire collection

            var ids = _.map(clusterList, function (cluster) {
                return 'cluster.' + cluster.id;
            });

            return getStatistics(request, statistic, ids);
        };

        var getNodesStatistics = function (request, statistic) {
            var nodeId = request.params.nid;
            var ids = ['node.' + nodeId];
            return getStatistics(request, statistic, ids);
        };

        function getEventsStatistics(request, statistic) {
            var data = [];
            var severityIds = [
                'events.critical.created',
                'events.warning.created',
                'events.info.created'
            ];

            var now = Math.floor(Date.now() / 1000); // current time in seconds

            var fromTimestamp = now + convertToSeconds(request.from || '-1d'); // defaults to one day ago
            var untilTimestamp = now + convertToSeconds(request.until || '0s'); // defaults to now
            var interval = convertToSeconds(request.interval || '1h'); // defaults to 1 hour

            _.forEach(severityIds, function (id, index) {
                data.push(
                    {
                        type: 'event_severity_history',
                        id: id,
                        attributes: {
                            datapoints: _.map(_.range(fromTimestamp, untilTimestamp, interval), function (timestamp) {
                                var value = (statistic && statistic.value) ? (statistic.value / index) : _.random(0, 35);
                                return [value, timestamp];
                            })
                        }
                    }
                );
            });

            return {"data": data};
        };

        var getStatistics = function (request, statistic, idList) {
            var statisticMetrics = statistic || {
                    'bandwidth.read': {min: 52428800, max: 5368709120},
                    'bandwidth.write': {min: 52428800, max: 5368709120},
                    'latency.read': {min: 100000000000, max: 120000000000},
                    'latency.write': {min: 100000000000, max: 120000000000},
                    'io.read': {min: 100000, max: 1000000},
                    'io.write': {min: 100000, max: 1000000},
                    'size': {min: 1024 * 1024 * 1024 * 1024, max: 30 * 1024 * 1024 * 1024 * 1024},
                    'cpu.idle': {min: 0, max: 100},
                    'mem.used': {min: 0, max: 30 * 1024 * 1024 * 1024 * 1024},
                    'mem.free': {min: 0, max: 30 * 1024 * 1024 * 1024 * 1024},
                    'temp': {min: 0, max: 100}
                };

            var now = Math.floor(Date.now() / 1000); // current time in seconds

            var fromTimestamp = convertToSeconds(_.defaultTo(request.query.from, '-5min'), now); // defaults to five minutes ago
            var untilTimestamp = convertToSeconds(_.defaultTo(request.query.until, '0s'), now); // defaults to now
            var intervalSeconds = convertToSeconds(_.defaultTo(request.query.interval, '10s')); // defaults to 10 seconds

            var re = new RegExp('\\bstorage_pool\\.' + cacheStoragePoolId + '\\b');
            var data = [];

            // make sure interval is a multiple of 10
            var intervalTens = Math.round(intervalSeconds / 10) * 10;

            // make sure interval has the minimum of 10 seconds
            var interval = Math.max(10, intervalTens);

            // make sure `fromTimestamp` is a multiple of 10
            fromTimestamp = Math.round(fromTimestamp / 10) * 10;

            _.forEach(idList, function (id, iterator) {
                _.forOwn(statisticMetrics, function (metricRange, metricName) {

                    // filter out entries according to request's query string `filter` parameter
                    if (!new RegExp(request.query.filter).test(id + '.' + metricName)) {
                        return true;
                    }

                    // for interfaces of containers - skip size metric
                    if (metricName === 'size' && _.includes(id.toLowerCase(), 'interface')) {
                        return true;
                    }

                    // size should be the only metric on the primary entity level (e.g.: avoid container.1024.latency.read)
                    if (metricName !== 'size' && id.split('.').length === 2 && !_.startsWith(id, 'node') && !_.startsWith(id, 'cluster')) {
                        return true;
                    }

                    // only node and cluster should have memory, temperature and cpu statistics
                    if (_.includes(['mem.used', 'mem.free', 'cpu.idle', 'temp'], metricName) && !_.startsWith(id, 'node') && !_.startsWith(id, 'cluster')) {
                        return true;
                    }
                    data.push({
                        type: 'statistics_history',
                        id: id + '.' + metricName,
                        attributes: {
                            datapoints: _.map(_.range(fromTimestamp, untilTimestamp, interval), function (timestamp) {

                                // for container's virtual "cache" storage pool - give larger factor to value
                                var factor = re.test(id) ? 3 : 1;

                                // get data-point's value by sampling a random number from its range and using the factor
                                var value;
                                if (request.query.projection == 'containers') {
                                    value = factor * (metricRange.max / (iterator + 1));
                                } else {
                                    value = factor * ((statistic) ? metricRange : _.random(metricRange.min, metricRange.max));
                                }
                                return [value, timestamp];
                            })
                        }
                    });
                });
            });

            return {"included": [], "data": data};
        };
    }


    /**
     * Creates an object that helps to generate Headers mocks
     * @constructor
     */
    function HeadersData() {
        /**
         * Returns generated mock of Headers data
         * @param {Object} object
         */
        this.generate = function (object) {
            for (var property in object) {
                defaultHeaders[property] = object[property];
            }

            return defaultHeaders;
        };

        // default headers data object
        var defaultHeaders = {
            "Access-Control-Allow-Credentials": true,
            "Access-Control-Allow-Origin": "http://127.0.0.1:8000",
            " Access-Control-Expose-Headers": "Content-Type, Content-Length, Last-Modified, x-igz-modified-author, x-amz-storage-class, x-igz-creation-date, x-igz-creation-author, x-igz-access-date, x-igz-access-author, x-igz-owner, x-igz-meta, x-igz-namespace, x-igz-tags, x-igz-description, x-igz-version, x-igz-type, x-igz-ranking, x-igz-properties, x-igz-profile, x-igz-attributes",
            "Connection": "keep-alive",
            "Content-Length": 8456,
            "Content-Type": "text/plain; charset=utf-8",
            "Last-Modified": "Fri Feb 05 2016 10:26:54 GMT+0200 (FLE Standard Time)",
            "Vary": "Origin",
            "x-amz-storage-class": "Unspecified",
            "x-igz-access-author": "Jimmy Page",
            "x-igz-access-date": "Fri Feb 05 2016 10:26:54 GMT+0200 (FLE Standard Time)",
            "x-igz-attributes": "string,company,Iguaz.io;string,year,2016",
            "x-igz-creation-author": "Robert Plant",
            "x-igz-creation-date": "Fri Feb 05 2016 10:26:54 GMT+0200 (FLE Standard Time)",
            "x-igz-description": "(No description)",
            "x-igz-meta": "-",
            "x-igz-modified-author": "Yaron Haviv",
            "x-igz-namespace": "iguazio-main",
            "x-igz-owner": "Yaron Haviv",
            "x-igz-profile": "my-data",
            "x-igz-properties": "compress,checksum,dedup,encrypt",
            "x-igz-ranking": 1,
            "x-igz-tags": "Tag BC",
            "x-igz-type": "DataSet",
            "x-igz-version": "1.0.2",
            "X-Powered-By": "Express"
        };
    }

    /**
     * Returns generated mock of resources list
     * @param {Object} request
     * @returns {{data: Array, included: Array}}
     */
    function resourcesList(request) {
        var results = {
            data: [],
            included: []
        };

        var resources = getEntitiesArray(request.resources);

        // filter results according to provided custom filter, if provided
        if (request.customFilter) {
            resources = _.filter(resources, request.customFilter);
        }

        if (!_.isUndefined(request.filter)) {
            resources = _.filter(resources, function (item) {
                return _.every(request.filter, function (predicates, field) {

                    // `predicates` might be an array of strings, or a single string
                    // convert to array in order to use common logic in all cases
                    return _.every(_.castArray(predicates), isPredicateSatisfied);

                    function isPredicateSatisfied(predicate) {
                        var operatorHandlers = {
                            'gt': _.gt,
                            'ge': _.gte,
                            'lt': _.lt,
                            'le': _.lte,
                            'eq': function (one, other) {
                                return _.includes(String(other).split(','), String(one));
                            },
                            'ne': function (one, other) {
                                return !_.includes(String(other).split(','), String(one));
                            },
                            'match': function (expression, regExp) {
                                return new RegExp(regExp).test(expression);
                            },
                            'match-i': function (expression, regExp) {
                                return new RegExp(regExp, 'i').test(expression);
                            }
                        };

                        // parse filter value as operator and value expression, i.e. [$operator]valueExpression
                        var matches = predicate.match(/^\[\$([\w\-]+)\](.*)$/);
                        var operator = _.get(matches, '[1]');
                        var valueExpression = _.get(matches, '[2]', predicate);

                        // get the relevant operator handler
                        // if no '[$operator]' is provided use 'eq' handler
                        // i.e. "?filter[name]=ab,ce" is equivalent to "?filter[name]=[$eq]ab,cd")
                        var operatorHandler = operatorHandlers[_.defaultTo(operator, 'eq')];

                        // if there is an operator provided, but the operator is invalid - respond with 400 Bad Request
                        if (!_.isNull(matches) && _.isUndefined(operatorHandler)) {
                            throw errors.invalidOperator;
                        }

                        // according to JSON-API "a resource can not have an attribute and relationship with the
                        // same name", so attributes and relationships can't get mixed up and retrieve undesirable
                        // results (i.e. retrieve an attribute instead of a relationship of the same name)
                        var itemValue;

                        // using `_.has` and not just `_.get` because values of attributes could be falsy (null, 0, '',
                        // false), and if we used: `_.get(attr) || _.get(rels)` the `||` operator would return
                        // `_.get(rels)` in case `_.get(attr)` returned a valid falsy value
                        if (_.has(item, 'attributes.' + field)) {
                            itemValue = _.get(item, 'attributes.' + field);
                        } else {
                            itemValue = _.get(item, 'relationships.' + field + '.data.id');
                        }
                        // attempt to convert values to numbers, and if that's not possible - convert to strings
                        itemValue = convertValue(itemValue);
                        valueExpression = convertValue(valueExpression);

                        // if no attribute/relationship matches the provided filter key - filter out this value
                        // otherwise filter by criteria
                        return operatorHandler(itemValue, valueExpression);
                    }

                    /**
                     * Attempts to get the provided value in its suitable type for operator
                     * @param {*} value
                     * @returns {boolean|number|string}
                     */
                    function convertValue(value) {
                        return typeof value === 'boolean' ? value              :
                                      value === 'true'    ? true               :
                                      value === 'false'   ? false              :
                                      _.defaultTo(Number(value), String(value));
                    }
                });
            });
        }

        var resourcesArr = [];

        _.forEach(resources, function (resource, index) {
            resourcesArr[index] = resource;

            for (var i = 1; i < Math.floor(request.count / resources.length); i++) {
                var newResource = _.cloneDeep(resource);
                newResource.id = _.isNumber(resource.id) ? Number((('1' + i).slice(-2)) + newResource.id.toString().substring(2)):
                                                           (('0' + i).slice(-2)) + newResource.id.toString().substring(2);
                newResource.attributes.name ? newResource.attributes.name = newResource.attributes.name + ' - ' + i : newResource.attributes.kind = newResource.attributes.kind + ' - ' + i;

                var pos = resources.length * i + index;
                resourcesArr[pos] = newResource;
            }
        });

        var parsed = {};
        _.forEach(resourcesArr, function (entity) {
            parsed[entity.id] = entity;
        });

        if (request.remove) {
            _.pullAt(resourcesArr, request.remove);
        }

        var editableResourcesArr = _.cloneDeep(resourcesArr);

        if (request.push) {
            if (_.isArray(request.push)) {
                _.forEach(request.push, function (entity) {
                    editableResourcesArr.push(entity)
                })
            } else {
                editableResourcesArr.push(request.push);
            }
        }

        if (_.has(request, 'edit')) {
            _.forEach(request.edit.attributes, function (value, key) {
                if (_.has(request.edit.attributes, key)) {
                    editableResourcesArr[_.findIndex(editableResourcesArr, {id: request.edit.id})].attributes[key] = value;
                }
            });
        }

        if (!_.isUndefined(request.sort)) {
            var sortList = _.trim(request.sort, ' ,').split(',');
            var sortByList = _.map(sortList, function (value) {
                return 'attributes.' + _.trimStart(value, '-');
            });
            var ordersList = _.map(sortList, function (value) {
                return _.startsWith(value, '-') ? 'desc' : 'asc';
            });
            editableResourcesArr = _.orderBy(editableResourcesArr, sortByList, ordersList);
        }

        if (_.has(request, 'page') || _.has(request, 'pageOf')) {
            var pageNumber = undefined ? 0 : request.page;
            var pageSize = request.perPage;
            var totalPages = _.ceil(editableResourcesArr.length / pageSize);

            if (!isNaN(pageSize)) {
                results.meta = {
                    'total_pages': totalPages === Infinity ? 0 : totalPages
                };
            }

            if (!_.isUndefined(request.pageOf)) {
                var resourceIndex = _.findIndex(editableResourcesArr, function (resource) {
                    return String(resource.id) === String(request.pageOf);
                });
                pageNumber = Math.floor(resourceIndex / pageSize);
                results.meta.page_number = pageNumber;
            }

            var firstElement = pageNumber * pageSize;
            var lastElement = _.min([firstElement + parseInt(pageSize), request.count]);
            editableResourcesArr = editableResourcesArr.slice(firstElement, lastElement);
        }

        _.forEach(editableResourcesArr, function (container) {
            results.data.push(container);
        });

        // omit invalid relationship paths (a relationship path is a dot-separated list of relationship names)
        var inclusionList = _.defaultTo(request.include, []).filter(function (relationshipPath) {
            return /(\w+\.)*\w+/.test(relationshipPath);
        });

        // prepare resources for response (filter fields if needed and include related resources if needed)
        results.data = _.isArray(editableResourcesArr) ? editableResourcesArr.map(prepareResourceWrapper) : prepareResourceWrapper(editableResourcesArr);

        function prepareResourceWrapper(resourceObject) {
            return prepareResource(resourceObject, inclusionList, request.fields, results.included);
        }

        return results;
    };

    /**
     * Creates an object that helps to generate Container mock
     * @constructor
     */
    function ContainerData() {

        this.generate = function (params_id, query_fields, query_include) {
            return getContainer(params_id, query_fields, query_include);
        };

        /**
         * Returns container by given container id (request.param), field (request.query.field),
         * include list - parsed to Array (request.query.include)
         * @param {string} params_id
         * @param {Object} query_fields = query.field
         * @param {Array} query_include = parsed to Array query.include
         * @returns {Object}
         */
        var getContainer = function (params_id, query_fields, query_include) {
            var container = containers[params_id];

            if (container) {
                _.assign(container.relationships, {
                    jobs: {
                        data: _.map(jobs, function (value) {
                            return _.pick(value, ['id', 'type']);
                        })
                    },
                    events: {
                        data: _.map(events, function (value) {
                            return _.pick(value, ['id', 'type']);
                        })
                    }
                });
            }
            return getResource({id: params_id}, {fields: query_fields, include: query_include}, 'container');
        };
    }

    function OverviewSessions() {

        /**
         * Returns overview sessions mock by given interface id (request.param.interface),  and include list - parsed to Array (request.query.include)
         * @param {string} interfaceId
         * @param {Array} inclusionList
         * @returns {Object}
         */
        this.generate = function (interfaceId, inclusionList) {
            return getInterface(interfaceId, inclusionList)
        };

        function getInterface(interfaceId, inclusionList) {
            var interfaceItem;
            var included = [];

            interfaceItem = _.find(interfaces, {
                "type": "interface",
                "id": interfaceId
            });

            prepareResource(interfaceItem, inclusionList, undefined, included);

            return {
                data: interfaceItem,
                included: included
            };
        }
    }


    /**
     * getResource
     * @param {Object} params
     * @param {Object} query
     * @param {string} type
     * @param {?Object} filters
     * @returns {Object}
     */
    var getResource = function (params, query, type, filters) {
        var resourceId = _.has(params, 'id') ? params.id : null;
        var primaryData;
        var included = [];
        var collection;
        var responseBody = {
            data: null
        };

        if (resourceId) {
            primaryData = getResourceByIdentifier({
                type: type,
                id: resourceId
            });
        } else {
            collection = collections[type];
            primaryData = collection ? _.values(collection) : null;

            if (filters) {
                primaryData = _.filter(primaryData, filters);
            }

            if (_.has(query, 'remove')) {
                _.pullAt(primaryData, query.remove);
            }

            if (_.has(query, 'push') && _.isArray(query.push)) {
                _.forEach(query.push, function (entity) {
                    primaryData.push(entity);
                });
            }

            if (!_.isUndefined(query.sort)) {
                var sortList = _.trim(query.sort, ' ,').split(',');
                var sortByList = _.map(sortList, function (value) {
                    if (query.sort === 'id' || query.sort === '-id') {
                        return _.trimStart(value, '-');
                    } else {
                        return 'attributes.' + _.trimStart(value, '-');
                    }
                });
                var ordersList = _.map(sortList, function (value) {
                    return _.startsWith(value, '-') ? 'desc' : 'asc';
                });
                primaryData = _.orderBy(primaryData, sortByList, ordersList);
            }

            // if `count` == ''records' - append the total number of records to the meta data of the response
            if (_.has(query, 'countRecords')) {
                _.set(responseBody, 'meta.total_records', primaryData.length);
            }

            if (_.has(query, 'page') || _.has(query, 'pageSize')) {
                var pageNumber = Number(_.get(query, 'page.number', 0));
                var pageSize = Number(_.get(query, 'page.size', 50));
                var totalPages = _.ceil(primaryData.length / pageSize);

                if (!isNaN(pageNumber) && !isNaN(pageSize)) {
                    _.set(responseBody, 'meta.total_pages', totalPages === Infinity ? 0 : totalPages);

                    var firstElement = pageNumber * pageSize;
                    var lastElement = _.min([firstElement + parseInt(pageSize), primaryData.length]);
                    primaryData = primaryData.slice(firstElement, lastElement);
                }
            }
        }

        // omit invalid relationship paths (a relationship path is a dot-separated list of relationship names)
        var inclusionList = _.defaultTo(query.include, []).filter(function (relationshipPath) {
            return /(\w+\.)*\w+/.test(relationshipPath);
        });

        // prepare resources for response (filter fields if needed and include related resources if needed)
        responseBody.data = _.isArray(primaryData) ? primaryData.map(prepareResourceWrapper) : prepareResourceWrapper(primaryData);

        // append included related resources to response
        responseBody.included = included;

        return responseBody;

        function prepareResourceWrapper(resourceObject) {
            return prepareResource(resourceObject, inclusionList, query.fields, included);
        }
    };

    var prepareResource = function (resource, inclusionList, fields, accumulator) {
        var result = _.defaultTo(accumulator, []);

        // the keys of `nextInclusionLists` represent the relationships in the current level that need to be included
        var nextInclusionLists = _.transform(inclusionList, function (newObject, path) {
            var steps = path.split('.');
            var current = _.first(steps);
            var next = _.tail(steps).join('.');

            if (!_.isArray(newObject[current])) {
                newObject[current] = [];
            }

            newObject[current].push(next);
        }, {});

        var fieldsForType = _.get(fields, resource.type); // get attribute names to pick from resource according to its type
        var attributes = _.isEmpty(fieldsForType) ?                     // if no list is provided for this type (or any)
            resource.attributes :                                       // pick all resource's attributes
            _.pick(resource.attributes, fieldsForType); // otherwise pick according to the specified list

        // pick the relationships that are requested to be included for this resource
        var relationships = _.pick(resource.relationships, _.keys(nextInclusionLists));

        // create a resource object to be returned (it is important not to modify the actual resource in "DB")
        var resourceObject = _.omitBy({
            type: resource.type,
            id: resource.id,
            attributes: _.cloneDeep(attributes),
            relationships: _.cloneDeep(relationships)
        }, _.overEvery(_.isObject, _.isEmpty)); // omit `attributes` or `relationships` object(s) if they are empty

        _.chain(relationships) // for each relationship that needs to be included
            .mapValues('data') // fetch the resource linkage
            .omitBy(_.isEmpty) // get rid of empty resource linkages: `null` for to-one, or `[]` for to-many
            .forEach(function (resourceLinkage, relationshipName) {

                // get the subset of inclusion list specific for the current relationship
                var nextInclusionList = nextInclusionLists[relationshipName];

                // resource linkage could be either a single resource identifier (Object) to indicate a to-one relationship,
                // or a list of multiple resource identifiers (Array of Objects) to indicate a to-many relationship.
                // in case it is a single object - wrap it inside an array to allow the next flow to always work the same.
                var resourceIdentifiers = _.castArray(resourceLinkage);

                // get the list of related resources themselves
                var relatedResources = _.chain(resourceIdentifiers)

                    // filter out resource identifiers that has already been added to the result array (preserve uniqueness)
                    .reject(function (resourceIdentifier) {
                        return _.some(result, resourceIdentifier);
                    })

                    // attempt to fetch the resource (identified by the resource identifier) in the relevant collection
                    .map(getResourceByIdentifier) // returns `null` if not found

                    // filter out resources that were not found
                    .reject(_.isNull)

                    // for each of the resources to include
                    .map(function (resourceToInclude) {

                        // recursively prepare resource
                        return prepareResource(resourceToInclude, nextInclusionList, fields, result);
                    })
                    .value();

                // append related resources to result array
                Array.prototype.push.apply(result, relatedResources);
            })
            .value();

        // also return the collected resources
        return resourceObject;
    };

    var updateBackRelations = function (updated, outdated) {
        var biDirectionalRelations = biDirectionalRelationships[updated.type];
        var updatedIdentifier = _.pick(updated, ['type', 'id']);
        outdated = _.isObject(outdated) ? outdated : {};

        _.forEach(biDirectionalRelations, function (relationship) {

            // get relationship data from both outdated and updated states of the resource
            var ownPath = 'relationships.' + relationship.own.name + '.data';
            var updatedRelationshipData = _.get(updated, ownPath);
            var outdatedRelationshipData = _.get(outdated, ownPath);

            // if there's data regarding the current relationship in the updated state of the resource
            if (!_.isUndefined(updatedRelationshipData)) {

                // if it's a to-many relationship
                if (relationship.own.asList) {

                    // each related resource that existed before (in the outdated state)
                    _.forEach(outdatedRelationshipData, function (resourceIdentifier) {

                        // but no longer exists now (in the updated state)
                        if (!_.some(updatedRelationshipData, resourceIdentifier)) {

                            // remove the updated resource from it
                            updateRelationship(resourceIdentifier, true); // true means "remove"
                        }
                    });

                    // for each related resource that exists now (in the updated state)
                    _.forEach(updatedRelationshipData, function (resourceIdentifier) {

                        // but did not exist before (in the outdated state)
                        if (!_.some(outdatedRelationshipData, resourceIdentifier)) {

                            // add the updated resource to it
                            updateRelationship(resourceIdentifier, false); // false means "add"
                        }
                    });
                }

                // if it's a to-one relationship
                else {

                    // if there was a change in the relationship (now relates to a different resource, or to no resource)
                    if (!_.isEqual(updatedRelationshipData, outdatedRelationshipData)) {

                        // if outdated state had a non-null related resource
                        if (!_.isNil(outdatedRelationshipData)) {

                            // remove its back relationship
                            updateRelationship(outdatedRelationshipData, true); // true means "remove"
                        }

                        // if updated resource now relates to a non-null resource
                        if (!_.isNull(updatedRelationshipData)) {

                            // update related resource to point back
                            updateRelationship(updatedRelationshipData, false); // false means "add"
                        }
                    }
                }
            }

            /**
             * Updates (adds or removes) a related resource's relationship with the updated resource.
             * @param {{type: string, id: string}} resourceIdentifier - identifies the related resource
             * @param {boolean} remove - determines whether the relationship with the updated resource should be removed
             * from the related resource, or added to it (true means remove, false means add)
             */
            function updateRelationship(resourceIdentifier, remove) {
                var related = getResourceByIdentifier(resourceIdentifier);
                var relatedPath = 'relationships.' + relationship.related.name + '.data';
                var relatedRelationshipData = _.get(related, relatedPath);

                // if it's a to-many relationship
                if (relationship.related.asList) {

                    // if it doesn't exist - create it
                    if (_.isUndefined(relatedRelationshipData)) {
                        _.set(related, relatedPath, remove ? [] : [resourceIdentifier]);
                    }

                    // if it exists - update it
                    else if (_.isArray(relatedRelationshipData)) {
                        if (remove) {
                            _.remove(relatedRelationshipData, updatedIdentifier);
                        }
                        else {

                            // before addition - make sure it doesn't already exist - to prevent duplicates
                            if (!_.some(relatedRelationshipData, updatedIdentifier)) {
                                relatedRelationshipData.push(updatedIdentifier);
                            }
                        }
                    }
                }
                // if it's a to-one relationship
                else {
                    _.set(related, relatedPath, remove ? null : updatedIdentifier);
                }
            }
        });
    };

    var getResourceByIdentifier = function (resourceIdentifier) {
        var collection = collections[resourceIdentifier.type];
        return collection ? getResourceById(collection, resourceIdentifier.id) : null;
    };

    var getResourceById = function (collection, id) {
        return (id in collection) ? collection[id] : null;
    };

    var randomString = function (length, chars) {
        var result = '';
        for (var i = length; i > 0; --i) {
            result += chars[Math.round(Math.random() * (chars.length - 1))];
        }
        return result;
    };

    var generateRandomId = function () {
        var result = "", arr = [8, 4, 4, 4, 12];
        arr.forEach(function (n) {
            result += "-" + randomString(n, "0123456789abcdef");
        });
        return result.substr(1); // skip first hyphen
    };

    function NewUserGroup(data, copy) {
        var newUserGroup = {
            "type": "user_group",
            "id": (data && data.id) ? data.id : generateRandomId(),
            "attributes": {
                "gid": 5,
                "name": (data && data.name) ? data.name : "new Group",
                "description": (data && data.description) ? data.description : "some descriptions",
                "enabled": (data && data.enabled) ? data.enabled : true,
                "created_at": currentDate.toISOString(),
                "assigned_policies": (data && data.policiesData) ? data.policiesData : []
            },
            "relationships": {
                "users": {
                    "data": (data && data.usersData) ? data.usersData : []
                }
            }
        };

        // add groups to the user

        var included = [];

        prepareResource(newUserGroup, ['groups', 'users', 'policies'], undefined, included);

        var newUserGroupData = {
            data: newUserGroup
        };

        if (copy) {
            newUserGroupData = newUserGroup;
        }

        return newUserGroupData
    };

    this.getStoragePoolDevices = function (poolID) {
        var includedData = [];
        var storage_pool = storagePools[poolID];

        var storage_pool_devices = storage_pool.relationships.storage_pool_devices.data;

        _.forEach(storage_pool_devices, function (storage_pool_device) {
            var includedPoolDevice = storagePoolDevices[storage_pool_device.id];
            var includedDevice = storageDevices[includedPoolDevice.relationships.storage_device.data.id];
            includedData.push(includedPoolDevice);
            includedData.push(includedDevice);
        })

        return includedData;
    };

    this.getPoolMock = function (params, query) {
        var storage_pool = storagePools[params.id];

        if (storage_pool) {
            var included = [];

            if (query.include) {
                var inclusionList = query.include;
                prepareResource(storage_pool, inclusionList, undefined, included);
            }

            return {
                data: storage_pool,
                included: included
            };
        }
    };

    this.getStoragePoolContainers = function (param, query) {
        var storagePoolId = param.storage_pool_id;
        var response;
        var filter = {
            "relationships": {
                "storage_pools": {
                    "data": [
                        {
                            "type": "storage_pool",
                            "id": storagePoolId
                        }
                    ]
                }
            }
        };

        // check that a storage pool id was sent and that there exists a storage pool with the specified id
        if (storagePoolId && _.has(storagePools, storagePoolId)) {
            response = getResource(param, query, 'container', filter);
        }

        return response;
    };

    this.getItemHandler = function (folder, attributesToGet) {
        var path = folder.put.url;
        var propertiesArr = [
            'compress',
            'compress,checksum',
            'checksum',
            'checksum,dedup,encrypt',
            'dedup',
            'compress,encrypt',
            'compress,checksum,dedup,encrypt'
        ];
        var allData = {
            '__name': {'S': pathModule.parse(path).base},
            '__size': {'N': 2048},
            '__mode': {'N': '33200'},
            '__atime_secs': {'N': 1475156740836},
            '__mtime_secs': {'N': 1475132735248},
            '__ctime_secs': {'N': 1445518112222},
            '__accessed_by_uid': folder.put.Item.__accessed_by_uid,
            '__modified_by_uid': folder.put.Item.__modified_by_uid,
            '__ranking': {'F': 5},
            '__thumbnail': folder.put.Item.__thumbnail,

            // todo will be implemented later
            'has_ssn': {'S': 'Some value'},
            'has_ccard': {'S': 'Some value'},
            'Height': {'S': 'Some value'},
            'Length': {'S': 'Some value'},
            'Width': {'S': 'Some value'},
            'Y-Resolution': {'S': 'Some value'},
            'X-Resolution': {'S': 'Some value'},
            'Latitude': {'S': 'Some value'},
            'Longitude': {'S': 'Some value'},
            'ScanTime': {'S': 'Some value'},
            'CustomAttribute1': {'N': 25},
            'CustomAttribute2': {'S': 'abcd'},
            'CustomAttribute3': {'S': '342432'},
            'CustomAttribute4': {'S': 'Some value542'},
            'CustomAttribute5': {'S': 'Some value54'},
            'CustomAttribute6': {'S': 'Some value13'},
            'CustomAttribute7': {'S': 'Some value6'},
            'CustomAttribute8': {'S': 'Some value32'},
            'CustomAttribute9': {'S': 'Some value3'},
            'CustomAttribute10': {'S': 'Some value1'},
            'CustomAttribute11': {'S': 'Some value2'},
            'CustomAttribute12': {'S': 'Some value4'},
            'CustomAttribute13': {'S': 'Some value5'},
            'CustomAttribute14': {'S': 'Some value7'},
            'CustomAttribute15': {'S': 'Some value8'},
            'CustomAttribute16': {'S': 'Some value9'},
            'CustomAttribute17': {'S': 'Some value10'},
            'CustomAttribute18': {'S': 'Some value11'},
            'CustomAttribute19': {'S': 'Some value12'},
            'CustomAttribute20': {'S': 'Some value13'},

            // todo temporary attrs for demo
            '__type': {'S': 'DataSet'},
            '__owner': {'S': 'Eran Duchan'},
            '__tags': {'S': ''},
            '__version': {'S': '1.0.2'},
            '__properties': {'S': propertiesArr[1]},
            '__mauthor': {'S': 'Eran Duchan'},
            '__cauthor': {'S': 'Amy Davidson'},
            '__aauthor': {'S': 'Jimmy Page'}
        };

        for (var property in folder.put.Item) {
            allData[property] = folder.put.Item[property];
        }
        // creates response data according to 'AttributesToGet' request property
        var data = attributesToGet ?
        {Item: _.pick(allData, attributesToGet.split(','))} :
        {Item: allData};

        return data;
    };

    function getGeneralVariables() {
        return {
            authors: [
                '1',
                '2',
                '3',
                '4',
                '5',
                '6',
                '7',
                '8'
            ],
            authorsMock: [
                'Yaron Haviv',
                'Eran Duchan',
                'Amy Davidson',
                'Joe McDonald',
                'Torin Torinyus',
                'Dori Dorinyus',
                'Jimmy Page',
                'Robert Plant'
            ],

            // max ranking rate value
            maxRanking: 5,

            // min ranking rate value
            minRanking: 0,

            propertiesArr: [
                'compression',
                'compression,checksum',
                'checksum',
                'checksum,dedup,encryption',
                'dedup',
                'compression,encryption',
                'compression,checksum,dedup,encryption'
            ],
            allData: {
                '__schema': {'S': '{"fields": [{"name": "n","type": "long","nullable": false},{"name": "str","type": "string","nullable": true}]}'},

                // todo will be implemented later
                'has_ssn': {'S': 'Some value'},
                'has_ccard': {'S': 'Some value'},
                'Height': {'S': 'Some value'},
                'Length': {'S': 'Some value'},
                'Width': {'S': 'Some value'},
                'Y-Resolution': {'S': 'Some value'},
                'X-Resolution': {'S': 'Some value'},
                'Latitude': {'S': 'Some value'},
                'Longitude': {'S': 'Some value'},
                'ScanTime': {'S': 'Some value'},

                'CustomAttribute1': {'N': 25},
                'CustomAttribute2': {'S': 'abcd'},
                'CustomAttribute3': {'S': '342432'},
                'CustomAttribute4': {'S': 'Some value542'},
                'CustomAttribute5': {'S': 'Some value54'},
                'CustomAttribute6': {'S': 'Some value13'},
                'CustomAttribute7': {'S': 'Some value6'},
                'CustomAttribute8': {'S': 'Some value32'},
                'CustomAttribute9': {'S': 'Some value3'},
                'CustomAttribute10': {'S': 'Some value1'},
                'CustomAttribute11': {'S': 'Some value2'},
                'CustomAttribute12': {'S': 'Some value4'},
                'CustomAttribute13': {'S': 'Some value5'},
                'CustomAttribute14': {'S': 'Some value7'},
                'CustomAttribute15': {'S': 'Some value8'},
                'CustomAttribute16': {'S': 'Some value9'},
                'CustomAttribute17': {'S': 'Some value10'},
                'CustomAttribute18': {'S': 'Some value11'},
                'CustomAttribute19': {'S': 'Some value12'},
                'CustomAttribute20': {'S': 'Some value13'},

                // todo temporary attrs for demo
                '__type': {'S': 'DataSet'},
                '__version': {'S': '1.0.2'}
            }
        }
    }

    this.getItemsHandler = function (folder, attributesToGet, pagination, filterExpression) {
        var path = folder.put.url;
        var limit = pagination.limit || 50;
        var allData = [];
        var files = [];
        var stats = folder.get.content;
        var data = {
            LastItemIncluded: "TRUE",
            NextMarker: pagination.marker,
            NumItems: allData.length,
            Items: null
        };
        _.forEach(stats, function (stat, key) {
            if (stat.Contents && stat.Contents.Key) {
                files.push(stats[key].Contents);
            }
        });

        //var allFilesPromise = new Promise(function (resolve, reject) {
        _.forEach(files, function (file) {
            var fileData = _.find(foldersArray, function (item) {
                return path + file.Key === item.get.url;
            });
            var tags = _.get(fileData, 'put.Item.__tags', {'S': ''});
            allData.push(_.merge(getGeneralVariables().allData, {
                '__name': {'S': file.Key},
                '__size': fileData.put.Item.__size,
                '__mode': {'N': '33200'},
                '__uid': fileData.put.Item.__uid,
                '__atime_secs': fileData.put.Item.__atime_secs,
                '__mtime_secs': fileData.put.Item.__mtime_secs,
                '__ctime_secs': fileData.put.Item.__ctime_secs,
                '__ranking': {'F': 5},
                '__thumbnail': fileData.put.Item.__thumbnail,

                '__owner': fileData.put.Item.__owner,
                '__tags': tags,
                //'__properties': fileData.put.Item.__properties,
                '__accessed_by_uid': fileData.put.Item.__accessed_by_uid,
                '__modified_by_uid': fileData.put.Item.__modified_by_uid
            }));

            if (allData.length === files.length) {


                // get system attributes only
                if (attributesToGet) {
                    if (attributesToGet === '**') {
                        data.Items = allData;
                    } else {
                        data.Items = _.map(allData, function (dataItem) {
                            return _.pick(dataItem, attributesToGet.split(','));
                        });
                    }
                }

                if (!_.isNil(filterExpression)) {
                    var filterQueries = filterExpression.split(' and ');
                    var dataFilterNames = ['__ctime_secs', '__mtime_secs', '__atime_secs', '__size', '__uid'];
                    var dataFilers = {};

                    _.forEach(dataFilterNames, function (dateFilterName) {
                        dataFilers[dateFilterName] = _
                            .chain(filterQueries)
                            .map(function (query) {

                                // remove data filter names from query string
                                return _.startsWith(query, dateFilterName) ? query.slice(dateFilterName.length + 1, query.length) : '';
                            })
                            .without('')
                            .value();
                    });

                    _.forEach(dataFilers, function (filterValues, filterKey) {
                        if (!_.isEmpty(filterValues)) {
                            var range = {};

                            data.Items = _.remove(data.Items, function (item) {
                                if (filterKey === '__uid' && !_.isNil(filterValues[0])) {

                                    // filterValues[0] in the form "=/in/>/< value"
                                    var usersId = _.without(filterValues[0].split(/in|==|\)|\(|,|\s+/), '');

                                    return _.includes(usersId, item[filterKey].S);
                                } else {

                                    // if we have custom between range first condition is invoked, if we have
                                    // custom from/to range second condition is invoked
                                    if (filterValues.length > 1) {
                                        _.forEach(filterValues, function (val) {
                                            switch (val[0]) {
                                                case '<':
                                                    range.to = val.slice(2, val.length);
                                                    break;
                                                case '>':
                                                    range.from = val.slice(2, val.length);
                                                    break;
                                            }
                                        });

                                        return item[filterKey].N >= Number(range.from) && item[filterKey].N <= Number(range.to);
                                    } else if (filterValues.length === 1) {
                                        switch (filterValues[0][0]) {
                                            case '<':
                                                range.to = filterValues[0].slice(2, filterValues[0].length);
                                                break;
                                            case '>':
                                                range.from = filterValues[0].slice(2, filterValues[0].length);
                                                break;
                                        }

                                        return item[filterKey].N >= Number(range.from) || item[filterKey].N <= Number(range.to);
                                    }
                                }
                            });
                        }
                    });
                }
            }
        });

        if (data.Items) {
            var startIndex = 0;
            if (pagination.marker !== '') {

                // finds index of marker
                startIndex = _.findIndex(data.Items, ['__name', {S: pagination.marker}]) + 1;
            }

            var lastIndex = startIndex + limit;

            // checks if the next bunch of data is exist
            data.LastItemIncluded = _.isUndefined(data.Items[lastIndex]) ? 'TRUE' : 'FALSE';

            // gets object keys, starting with key after the marker in order
            data.Items = data.Items.slice(startIndex, lastIndex);

            if (data.LastItemIncluded === 'FALSE') {
                data.NextMarker = _.get(_.last(data.Items), '__name.S', '');
            }

            data.NumItems = data.Items.length;
        }

        return data;
    };

    this.putItemHandler = function (folder, putAttributes) {
        var key = folder.key;

        var data = {
            Key: key,
            Item: putAttributes
        };

        return data;
    };

    this.tenantHandler = function (tenantId, conflictStatus) {
        if (conflictStatus === "no-conflict") {
            var data = {
                "data": {
                    "type": "job",
                    "id": "6ed5ed39-d234-4b14-a1a2-e57a47bcdcc8",
                    "attributes": {
                        "kind": "idp.synchronize.hierarchy",
                        "params": "",
                        "delay": 0,
                        "state": "completed",
                        "created_at": "2017-03-17T14:21:34.275Z",
                        "updated_at": "2017-03-17T14:21:34.276Z"
                    },
                    "relationships": {
                        "tenant": {
                            "data": {
                                "type": "tenant",
                                "id": tenantId
                            }
                        }
                    }
                }
            }
        } else if (conflictStatus === "conflict") {
            var data = {
                "data": {
                    "type": "job",
                    "id": "6ed5ed39-d234-4b14-a1a2-e57a47bcdcc8",
                    "attributes": {
                        "kind": "idp.synchronize.hierarchy",
                        "params": "",
                        "delay": 0,
                        "state": "completed",
                        "created_at": "2017-03-17T14:21:34.275Z",
                        "updated_at": "2017-03-17T14:21:34.276Z",
                        "result": syncResult
                    },
                    "relationships": {
                        "tenant": {
                            "data": {
                                "type": "tenant",
                                "id": tenantId
                            }
                        }
                    }
                }
            }
        } else {
            var data = {
                "data": {
                    "type": "job",
                    "id": "6ed5ed39-d234-4b14-a1a2-e57a47bcdcc8",
                    "attributes": {
                        "kind": "idp.synchronize.hierarchy",
                        "params": "",
                        "delay": 0,
                        "state": "in_progress",
                        "created_at": "2017-03-17T14:21:34.275Z",
                        "updated_at": "2017-03-17T14:21:34.276Z"
                    },
                    "relationships": {
                        "tenant": {
                            "data": {
                                "type": "tenant",
                                "id": tenantId
                            }
                        }
                    }
                }
            }
        }
        return data;
    };

    var syncResult = '{\"users\": {\"deleted\": 1, \"updated\": 1, \"conflicted_users_report\": {\"uid_conflicts\": ' +
        '[\"testuser6\"], \"total\": 4, \"users_in_conflicting_groups\": [\"testuser4\", \"testuser5\"], ' +
        '\"username_conflicts\": [\"testuser2\"]}, \"created\": 3}, \"groups\": {\"deleted\": 1, \"conflicts\": ' +
        '{\"total\": 2, \"gid\": [\"GidConflictingGroup\"], \"name\": [\"ConflictingGroup\"]}, \"updated\": 0, ' +
        '\"created\": 2}}';

    function NewContainer(data) {
        var newContainer = {
            "type": "container",
            "id": (data && data.containerId) ? data.containerId : 'b539b1f3-ae41-4388-9201-f0f083d92c4n',
            "attributes": {
                "name": data.name,
                "description": data.description,
                "num_objects": 'under_100m',
                "created_at": '2017-04-12T06:48:42.430Z',
                "quota": 200,
                "admin_status": "down",
                "operational_status": "down",
                "cost": 25,
                "limits_total_iops": 2,
                "limits_total_bandwidth_value": 3,
                "limits_total_bandwidth_unit": "Gb/s",
                "max_iops": 1800,
                "max_bandwidth": 2926298,
                "io_units": 25,
                "latency_target": {
                    "percentage_under": 60,
                    "latency_us": 0
                },
                "features": (data && data.features) ? data.features : {
                    "encryption": false,
                    "compression": false,
                    "dedup": false,
                    "checksum": false
                },
                "data_policy_layers_order": [],
                "data_lifecycle_layers_order": [],
                "capacity_chart": {
                    "series": [
                        {
                            "name": "Memory",
                            "y": 43
                        },
                        {
                            "name": "Performance",
                            "y": 19
                        },
                        {
                            "name": "Capacity",
                            "y": 29
                        },
                        {
                            "name": "Cold",
                            "y": 9
                        }
                    ],
                    "total_value": 232783872
                },
                "iops_chart": {
                    "series": [
                        {
                            "name": "Memory",
                            "y": 15
                        },
                        {
                            "name": "Performance",
                            "y": 36
                        },
                        {
                            "name": "Capacity",
                            "y": 17
                        },
                        {
                            "name": "Cold",
                            "y": 32
                        }
                    ],
                    "total_value": 491000
                },
                "snapshots": [
                    {
                        "created_at": '2017-04-12T06:48:42.430Z'
                    }
                ]
            },
            "relationships": {
                "default_storage_pool": {
                    "data": {
                        "type": "storage_pool",
                        "id": _.find(_.toArray(storagePools), {attributes: {name: data.default_storage_pool || "AWS-S3"}}).id
                    }
                },
                "workload_profile": {
                    "data": {
                        "type": "workload_profile",
                        "id": _.find(_.toArray(workloadProfiles), {attributes: {name: data.workload_profile || "Hadoop"}}).id
                    }
                },
                "storage_class": {
                    "data": {
                        "type": "storage_class",
                        "id": _.find(_.toArray(storageClasses), {attributes: {name: data.storage_class || "Capacity"}}).id
                    }
                },
                "owner": {
                    "data": {
                        "type": "user",
                        "id": "76b6fd2e-be70-477c-8226-7991757c0d09"
                    }
                },
                "interfaces": {
                    "data": [
                        {
                            "type": "interface",
                            "id": _.find(_.toArray(interfaces), {attributes: {kind: data.interfaces || "web"}}).id
                        }
                    ]
                },
                "data_policy_layers": {
                    "data": []
                },
                "data_lifecycle_layers": {
                    "data": []
                },
                "events": {
                    "data": []
                },
                "tenant": {
                    "data": {
                        "type": "tenant",
                        "id": "21345678-1231-5671-1224-567812345438"
                    }
                }
            }
        };
        containers[data.containerId] = newContainer;
        return newContainer;
    }

    function NewNetwork(data) {
        var newNetwork = {
            "type": "network",
            "id": (data && data.id) ? data.id : "3031fc3b-44c1-ab2c-ea4a-153649bc9d26",
            "attributes": {
                "name": data.name,
                "description": (data && data.description) ? data.description : "Description",
                "subnet": (data && data.subnet) ? data.subnet : "127.0.0.1",
                "mask": (data && data.mask) ? data.mask : "255.255.255.0",
                "default_gateway": (data && data.default_gateway) ? data.default_gateway : "0.0.0.0",
                "kind": (data && data.kind) ? data.kind : "untagged",
                "tag": (data && data.tag) ? data.tag : "123",
                "created_at": (data && data.created_at) ? data.created_at : "2016-01-27T09:26:29.045Z"
            },
            "relationships": {
                "tenant": {
                    "data": {
                        "type": "tenant",
                        "id": "12345678-1234-5678-1234-567812345678"
                    }
                }
            }
        };
        return newNetwork;
    }

    function NewEscalation(data) {
        var newEscalation = {
            "type": "escalation_policy",
            "id": (data && data.id) ? data.id : "be7d1e92-25f7-43e6-9940-9a8336841c62",
            "attributes": {
                "name": data.name,
                "max_repetitions": 0,
                "escalation_suspension_delay_following_ack": 0,
                "rule_order": [],
                "created_at": "2016-09-13T08:34:23.726Z"
            },
            "relationships": {
                "rules": {
                    "data": []
                },
                "filters": {
                    "data": []
                }
            }
        };
        return newEscalation
    }

    function NewUser(data) {
        var newUser = {
            "data": {
                "type": "user",
                "id": (data && data.id) ? data.id : '66b6fd2e-be70-477c-8226-7991757c0d00',
                "attributes": {
                    "uid": (data && data.uid) ? data.uid : 9,
                    "first_name": (data && data.first_name) ? data.first_name : "FirstName",
                    "last_name": (data && data.last_name) ? data.last_name : "LastName",
                    "username": (data && data.username) ? data.username : "username",
                    "password": (data && data.password) ? data.password : "19939266-955c-78b9-0ba5-72473da39b28",
                    "email": (data && data.email) ? data.email : null,
                    "phone_number": (data && data.phone_number) ? data.phone_number : null,
                    "job_title": (data && data.job_title) ? data.job_title : null,
                    "department": (data && data.department) ? data.department : null,
                    "password_changed_at": currentDate.toISOString(),
                    "last_activity": "2016-02-17T15:17:33.289Z",
                    "created_at": currentDate.toISOString(),
                    "enabled": (data && data.enabled) ? data.enabled : true,
                    "avatar": (data && data.avatar) ? ("image/png;base64," + data.avatar) : null
                },
                "relationships": {
                    "user_groups": {
                        "data": (data && data.groupsData) ? data.groupsData : []
                    }
                }
            }
        };
        return newUser;
    }

    function NewTenant(data) {
        var newTenant = {
            "data": {
                "type": "tenant",
                "id": (data && data.id) ? data.id : '055ac8bf-dffd-4be5-8d36-65b330f5b221',
                "attributes": {
                    "name": data.name,
                    "created_at": currentDate.toISOString(),
                    "updated_at": currentDate.toISOString(),
                    "default":false,
                    "visibility":"all"
                },
                "included":[]
            }
        };
        return newTenant;
    }

    function PasswordReset(data) {
        var user = _.find(users, ['id', data.userId]);
        var PasswordReset = {
            "data": {
                "type": "password_reset",
                "id": generateRandomId(),
                "attributes": {
                    "created_at": currentDate.toISOString(),
                    "updated_at": currentDate.toISOString(),
                    "ttl": 3600
                },
                "relationships": {
                    "user": {
                        "data": {
                            "type": 'user',
                            "id": user.id
                        }
                    },
                    "tenant": user.relationships.tenant
                }
                //,"included": user
            }
        };
        var collection = PasswordReset && collections[PasswordReset.data.type];
        collection[PasswordReset.data.id] = PasswordReset.data;
        return PasswordReset;
    }

    function NewStoragePool(data) {
        var storagePool = {
            "data": {
                "type": "storage_pool",
                "id": data.id,

                "attributes": {
                    "name": data.name,
                    "created_at": "2015-10-04T17:07:48.509Z",
                    "path": data.url,
                    "username": "userName",
                    "password": null,
                    "description": data.description,
                    "ownership": data.ownership,
                    "operational_status": "up",
                    "redundancy_level": 4,
                    "features": (data && data.features) ? data.features : {
                        "encryption": false,
                        "compression": false,
                        "dedup": false,
                        "checksum": false
                    },
                    "exceed_on": "2016-07-29T17:07:48.509Z"
                },
                "relationships": {
                    "storage_class": {
                        "data": {
                            "type": "storage_class",
                            "id": "2f6016a0-bd46-4358-b400-935f92b6bf80"
                        }
                    },
                    "owner": {
                        "data": {
                            "type": "user",
                            "id": "66b6fd2e-be70-477c-8226-7991757c0d79"
                        }
                    }
                }
            }
        };
        return storagePool;
    }

    function NewSession(sessionData, updatedUser) {
        var user = updatedUser ? updatedUser : _.find(_.toArray(users), {id: sessionData.userID});
        var session = {"type": "session", "attributes": {"username": "Unauthorized", "password": "Unauthorized", "plane": "control"}};
        if (user.attributes.username === sessionData.username && user.attributes.password === sessionData.password) {
            var relationships = {
                "user": {
                    "data": {
                        "type": "user",
                        "id": sessionData.userID
                    }
                },
                "tenant": {
                    "data": {
                        "type": "tenant",
                        "id": user.relationships.tenant.data.id
                    }
                }
            };

            if (_.has(sessionData.relationships, 'userId')) {
                delete relationships['user']
            }

            if (_.has(sessionData.relationships, 'tenantId')) {
                delete relationships['tenant']
            }

            session = {
                "type": "session",
                "id": sessionData.sessionId,
                "attributes": {
                    "created_at": currentDate.toISOString(),
                    "updated_at": currentDate.toISOString(),
                    "group_ids": user.relationships.user_groups.data,
                    "uid": 0,
                    "gids": [0],
                    "expires_at": 0,
                    "plane": "control",
                    "ttl": 86400
                },
                "relationships": relationships
            };
        }
        return session;
    }

    var createJob = function (kind, containerId, tenantId, jobFunc, config) {
        var jobsTimeouts = {};
        config = _.defaults({}, config, {
            params: '',
            delay: 0,
            state: 'created',
            durationMin: 10000,
            durationMax: 15000,
            successRate: 100,
            successResult: '',
            failureResult: ''
        });
        var now = new Date().toISOString();
        var newJob = {
            type: 'job',
            id: generateRandomId(),
            attributes: {
                kind: kind,
                params: config.params,
                delay: config.delay,
                state: config.state,
                created_at: now,
                updated_at: now
            },
            relationships: {
                tenant: { data: { type: 'tenant', id: tenantId } }
            }
        };

        if (containerId) {
            newJob.relationships.container = { data: { type: 'container', id: containerId } };
        }

        var randomDuration = _.random(config.durationMin, config.durationMax);

        jobs[newJob.id] = newJob;
        jobsTimeouts[newJob.id] = null;

        if (newJob.attributes.state === 'created') {
            jobsTimeouts[newJob.id] = setTimeout(function () {
                newJob.attributes.state = 'in_progress';
                newJob.attributes.updated_at = new Date().toISOString();
                if (_.isFunction(jobFunc)) {
                    jobFunc()
                        .then(function (result) {
                            _.assign(newJob.attributes, {
                                result: _.defaultTo(result, config.successResult),
                                state: 'completed',
                                updated_at: new Date().toISOString()
                            });
                        })
                        .catch(function (result) {
                            _.assign(newJob.attributes, {
                                result: _.defaultTo(result, config.failureResult),
                                state: 'failed',
                                updated_at: new Date().toISOString()
                            });
                        });
                } else {
                    jobsTimeouts[newJob.id] = setTimeout(function () {
                        if (newJob.attributes.state === 'in_progress') { // make sure it wasn't canceled
                            var isSuccessful = _.random(1, 100) <= _.clamp(config.successRate, 0, 100);
                            _.assign(newJob.attributes, {
                                result: isSuccessful ? config.successResult : config.failureResult,
                                state: isSuccessful ? 'completed' : 'failed'
                            });
                        }
                    }, randomDuration);
                }
            }, newJob.attributes.delay * 1000);
        }
        return newJob;
    };

    this.postArtifactBundle = function () {
        var newArtifactBundle = {
            type: 'artifact_bundle',
            id: artifactBundles.length,
            attributes: {
                gatherings: _.chain(clusterNodes)
                    .map('attributes.name')
                    .compact()
                    .map(function (nodeName) {
                        var newJob = createJob('artifact.gathering.' + nodeName, null, '12345678-1234-5678-1234-567812345678', null, {
                            durationMin: 500,
                            durationMax: 1000,
                            successResult: JSON.stringify({
                                archive_path: 'volumes/logs/artifacts/' +
                                new Date().toISOString().replace(/:/g, '.') + '_' + nodeName +
                                '_artifact_bundle.7z',
                                publishing_job_id: ''
                            })
                        });
                        return {
                            job_id: newJob.id,
                            node_name: nodeName
                        };
                    })
                    .value()
            }
        };
        artifactBundles[newArtifactBundle.id] = newArtifactBundle;

        return {
            included: [],
            data: newArtifactBundle
        };
    };
};
module.exports = function () {
    var mockFormatter = e2e_imports.testUtil.mockFormatter();
    var constants = e2e_imports.testUtil.constants();
    var fs = require('fs');

    var containerId = 748911;
    var defaultUserId = '66b6fd2e-be70-477c-8226-7991757c0d79';
    var sessionId = constants.sessionId;

    this.getStatisticMetrics = function (iteratorForStatistic) {
        return {
            'bandwidth.read': 5368709120 / iteratorForStatistic,
            'bandwidth.write': 5368709120 / iteratorForStatistic,
            'latency.read': 120000000000 / iteratorForStatistic,
            'latency.write': 120000000000 / iteratorForStatistic,
            'io.read': 1000000 / iteratorForStatistic,
            'io.write': 1000000 / iteratorForStatistic,
            'size': 30 * 1024 * 1024 * 1024 * 1024 / iteratorForStatistic,
            'cpu.idle': Math.round(100 / iteratorForStatistic),
            'mem.used': Math.round(30 * 1024 * 1024 * 1024 * 1024 / iteratorForStatistic),
            'mem.free': Math.round(30 * 1024 * 1024 * 1024 * 1024 / iteratorForStatistic),
            'temp': Math.round(100 / iteratorForStatistic)
        }
    };

    /**
     * Get Login
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {data: {id: string, attributes: {ttl: number}, relationships: {user: {type: string, id: string}}}, included: *[]}}}}
     */
    this.POST_mockBackend_API_sessions = function (request, updatedUser) {
        var updatedUser = updatedUser ? updatedUser.response.data.data : null;
        var userID = (request && request.userId) ? request.userId : defaultUserId;
        var username = (request && request.username) ? request.username : 'erand';
        var password = (request && request.password) ? request.password : 'Password123456789';
        var relationships = (request && request.relationships) ? request.relationships : null;
        var sessionData = mockFormatter.generateSession({userID: userID, sessionId: sessionId, username: username, password: password, relationships: relationships}, updatedUser);
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/sessions'
            },
            response: {
                status: sessionData.id ? 201 : 401,
                data: {
                    "included": [],
                    "data": sessionData,
                    "errors": sessionData.id ? [{status: 201}] : [{status: 401, "title":"Unauthorized","detail":"Unauthorized login"}]
                }
            }
        };
    };

    this.GET_mockBackend_API_sessions = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/sessions?count=records&page%5Bsize%5D=0'
            },
            response: {
                status: constants.sessionId ? 201 : 304,
                data: {
                    data:[],
                    meta:{"total_pages":0,"total_records":1}
                }
            }
        }
    };

    this.GET_mockBackend_job_id = function (id) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/jobs/' + id
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({id: id}, {}, 'job')
            }
        }
    };

    this.POST_mockBackend_artifact_bundles = function () {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/artifact_bundles'
            },
            response: {
                status: 200,
                data: mockFormatter.postArtifactBundle()
            }
        }
    };

    this.POST_mockBackend_container_create = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/containers'
            },
            response: {
                status: 201,
                data: mockFormatter.generateNewContainer(data)
            }
        }
    };

    /**
     * Delete resource mock
     * @param {string} resourceId
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {}}}}
     * @constructor
     */
    this.DELETE_mockBackend_API_resources = function (request) {
        return {
            request: {
                method: 'DELETE',
                url: configURL.api.baseUrl + '/' + request.resources.replace('-', '_') + '/' + request.id
            },
            response: {
                status: 204,
                data: {}
            }
        }
    };

    this.GET_mockBackend_API_tasks = function (request) {
        var filter = '';

        _.forEach(request.filter, function (key, value) {
            if (value != 'created_at') {
                filter += 'filter%5B' + value + '%5D=' + encodeURI(key).replace(/\$/g, '\\$') + '&';
            }
        });

        return {
            request: {
                method: 'GET',
                url: '/' + configURL.api.baseUrl.replace(/\\/, '\/') + '\/jobs\\?filter%5Bcreated_at%5D=%5B\\$ge%5D(\\d{4})-(\\d{2})-(\\d{2})T((\\d{2}):(\\d{2}):(\\d{2}))\\.(\\d{3})Z&' + filter + 'page%5Bnumber%5D=' + request.page + '&page%5Bsize%5D=' + request.perPage + '&sort=' + request.sort + '/'
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList(request)
            }
        }
    };

    this.GET_mockBackend_resources_list_count = function (request) {
        var requestUrlString = '/' + request.resources.replace('-', '_') + '?';

        if (_.has(request, 'filter')) {
            _.forEach(request.filter, function (value, key) {
                if (_.isArray(value)) {
                    requestUrlString += 'filter%5B' + key + '%5D=' + encodeURI(value[0]) + '&filter%5B' + key + '%5D=' + encodeURI(value[1]) + '&';
                } else {
                    requestUrlString += 'filter%5B' + key + '%5D=' + encodeURI(value) + '&';
                }
            });
        }

        if (request.include && request.include.length > 0) {
            requestUrlString += 'include=' + request.include.join(',') + '&'
        }

        if (_.has(request, 'pagination')) {
            requestUrlString += 'pagination=' + request.pagination
        }

        if (_.has(request, 'page')) {
            requestUrlString += 'page%5Bnumber%5D=' + request.page + '&'
        }

        if (_.has(request, 'pageOf')) {
            requestUrlString += 'page%5Bof%5D=' + request.pageOf + '&'
        }

        if (_.has(request, 'perPage')) {
            requestUrlString += 'page%5Bsize%5D=' + request.perPage
        }

        if (_.has(request, 'sort')) {
            requestUrlString += '&sort=' + request.sort
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList(request)
            }
        }
    };

    this.GET_mockBackend_containers_include_owner_pool_workloadProfile_interfaces_network = function (include) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers?include=' + include.join(',')
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList({resources: 'containers'})
            }
        }
    };

    /**
     * Gets all resources
     * @param {string} resource
     * @returns {{request: {method: string, url: string}, response: {status: number, data: *[]}}}
     * @constructor
     */
    this.GET_mockBackend_resources = function (resource) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/' + resource.replace(/-/g, '_')
            },
            response: {
                status: 200,
                data: mockFormatter.getMockData(resource)
            }
        }
    };

    this.GET_mockBackend_container_info = function (includeList, id) {
        id = _.defaultTo(id, containerId);
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + id + '?include=' + includeList.join(',')
            },
            response: {
                status: 200,
                data: mockFormatter.generateContainer(id,
                    undefined,
                    includeList
                )
            }
        }
    };

    this.POST_mockBackend_API_escalation_policies = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/escalation_policies'
            },
            response: {
                status: 201,
                data: {
                    'data': mockFormatter.generateNewEscalation(data),
                    "included": []
                }
            }
        }
    };

    /**
     * Get lifecycle container info
     * @returns {Object}
     */
    this.GET_mockBackend_lifecycle_container_info = function (id) {
        id = _.defaultTo(id, containerId);
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + id + '?fields%5Bcontainer%5D=data_lifecycle_layers,name&include=data_lifecycle_layers,data_lifecycle_layers.rules,data_lifecycle_layers.rules.data_filter_categories,data_lifecycle_layers.rules.destination_storage_pool,data_lifecycle_layers.rules.store_policy_workload_profile,data_lifecycle_layers.rules.created_by,data_lifecycle_layers.rules.last_saved_by,data_lifecycle_layers.rules.destination_container'
            },
            response: {
                status: 200,
                data: mockFormatter.generateContainer(id,
                    {container: ['data_lifecycle_layers', 'name']},
                    [
                        'data_lifecycle_layers',
                        'data_lifecycle_layers.rules',
                        'data_lifecycle_layers.rules.data_filter_categories',
                        'data_lifecycle_layers.rules.destination_storage_pool',
                        'data_lifecycle_layers.rules.store_policy_workload_profile',
                        'data_lifecycle_layers.rules.created_by',
                        'data_lifecycle_layers.rules.last_saved_by',
                        'data_lifecycle_layers.rules.destination_container'
                    ]
                )
            }
        }
    };

    /**
     * Get all networks
     * @param {?Object} query
     * @returns {{request: {method: string, url: string}, response: {status: number, data: *[]}}}
     * @constructor
     */
    this.GET_mockBackend_networks = function (query) {
        var query = query ? query : {};
        query.include = ['tenant'];
        var requestUrlString = '/networks?include=' + query.include.join(',');

        if (_.has(query, 'page')) {
            requestUrlString += '&page%5Bnumber%5D=' + query.page.number + '&page%5Bsize%5D=' + query.page.size;
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString + (query.sort ? '&sort=' + query.sort : '')
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({}, query, 'network')
            }
        }
    };

    /**
     * Gets all storage_pools include=owner
     * @param {?Object} query
     * @returns {{request: {method: string, url: string}, response: {status: number, data: object}}}
     * @constructor
     */
    this.GET_mockBackend_storage_pools_include_owner = function (query) {
        var urlString = '/storage_pools?';

        if (query.field) {
            urlString += query.field + '&';
        }

        if (query.include && query.include.length > 0) {
            urlString += 'include=' + query.include.join(',');
        }

        if (query.page) {
            urlString += '&page%5Bnumber%5D=' + query.page.number + '&page%5Bsize%5D=' + query.page.size;
        }

        if (query.sort) {
            urlString += '&sort=' + query.sort;
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + urlString
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({}, query, 'storage_pool')
            }
        }
    };

    this.GET_mockBackend_storage_pools_id = function (storage_pool_id) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/storage_pools/' + storage_pool_id
            },
            response: {
                status: 200,
                data: mockFormatter.getPoolMock({id: storage_pool_id}, {}, 'storage_pool')
            }
        }
    };

    this.GET_mockBackend_storage_pools_id_include_owner = function (storage_pool_id, query) {
        var urlString = '/storage_pools/' + storage_pool_id + (query ? '?' : '');

        if (query.field) {
            urlString += query.field + '&';
        }

        if (query.include && query.include.length > 0) {
            urlString += 'include=' + query.include.join(',');
        }

        if (query.page) {
            urlString += 'page%5Bnumber%5D=' + query.page.number + '&page%5Bsize%5D=' + query.page.size;
        }

        if (query.sort) {
            urlString += '&sort=' + query.sort;
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + urlString
            },
            response: {
                status: 200,
                data: mockFormatter.getPoolMock({id: storage_pool_id}, {include: query.include}, 'storage_pool')
            }
        }
    };

    this.GET_mockBackend_storage_pools_fields_include_events = function (storage_pool_id) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/storage_pools/' + storage_pool_id + '?fields%5Bstorage_pool%5D=id&include=events'
            },
            response: {
                status: 200,
                data: mockFormatter.getPoolMock({id: storage_pool_id}, {fields: {storage_pool: "id"}, include: ['events']}, "storage_pool")
            }
        };
    };

    this.GET_mockBackend_storage_pools_containers_include_owner = function (storage_pool_id, query) {
        query = query || {};
        if (_.isUndefined(query.include)) {
            query.include = ['owner']
        }

        var requestUrlString = '/storage_pools/' + storage_pool_id + '/containers?' + 'include=' + query.include.join(',');

        if (_.has(query, 'page')) {
            if (!query.sort) {
                query.sort = 'name'
            }
            requestUrlString += '&page%5Bnumber%5D=' + query.page.number + '&page%5Bsize%5D=' + query.page.size + '&sort=' + query.sort
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.getStoragePoolContainers({storage_pool_id: storage_pool_id}, query)
            }
        }
    };

    this.GET_mockBackend_storage_pools_devices = function (storage_pool_id, query) {
        var queryObj = {};
        var requestUrlString = '/storage_pools/' + storage_pool_id + '?fields%5Bstorage_pool%5D=storage_pool_devices&include=storage_pool_devices.storage_device';

        if (query && _.has(query, 'page')) {
            queryObj.page = query.page;
            requestUrlString += '&page%5Bnumber%5D=' + query.page + '&page%5Bsize%5D=' + query.perPage + (query.sort ? '&sort=' + query.sort : '');
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({id: storage_pool_id}, {include: ['storage_pool_devices.storage_device']}, 'storage_pool', undefined)
            }
        };
    };

    /**
     * Gets all access_control_datetime_ranges
     * @returns {{request: {method: string, url: string}, response: {status: number, data: *[]}}}
     * @constructor
     */
    this.POST_mockBackend_access_control_datetime_ranges = function () {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/data_policy_datetime_ranges'
            },
            response: {
                status: 200,
                data: {
                    "data": {
                        "type": "data_policy_datetime_range",
                        "attributes": {
                            "name": "newRange",
                            "created_at": "2016-02-11T11:59:52.651Z"
                        },
                        "id": "1f292859-e2d4-cbd2-cbc6-cec021f25b17"
                    }
                }
            }
        }
    };

    this.foldersArray = require('./mock-data.repository/folders.json');

    // /**
    //  * Get folder content
    //  * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
    //  */
    // this.GET_mockS3_folder_max_keys = function (element, id) {
    //     id = _.defaultTo(id, containerId);
    //     var url = configURL.nginx.baseUrl + '/' + id + '/';
    //     return {
    //         request: {
    //             method: 'GET',
    //             url: url
    //         },
    //         response: {
    //             status: (element ? 200 : 401),
    //             data: (element ? mockFormatter.generateXml(element.resources.get.content, element, headers) : null);
    //         }
    //     }
    // };

    /**
     * Get folder content with '&prefix-info=1'
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockS3_folder_prefix_only = function (element, id) {
        id = _.defaultTo(id, containerId);
        if (element) {
            var url = configURL.nginx.baseUrl + '/' + id + '/?delimiter=%2F&prefix=' + _.trimEnd(element.resources.get.url, '/').replace(/\//g, '%2F').replace(/ /g, '%2520') + '&prefix-only=1';
        } else {
            var url = configURL.nginx.baseUrl + '/' + id + '/?delimiter=%2F&prefix=&prefix-only=1';
        }
        return {
            request: {
                method: 'GET',
                url: url
            },
            response: {
                status: (element ? 200 : 401),
                data: (element ? mockFormatter.generateXml(element.resources.get.content, element, {'prefix-only':'1'}) : null)
            }
        }
    };

    /**
     * Get folder content with '&prefix-info=1'
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockS3_folder_max_keys_prefix_info = function (element, id) {
        id = _.defaultTo(id, containerId);
        if (element) {
            var params = _.defaultTo(element.resources.xmlParams, {'prefix-info': 1, 'max-keys': 50});
            var url = configURL.nginx.baseUrl + '/' + id + '/?delimiter=%2F&marker=' + (params['marker'] || '') +
                '&max-keys=' + (params['max-keys'] || 50) + '&prefix=' +
                _.trimEnd(element.resources.get.url, '/').replace(/\//g, '%2F').replace(/ /g, '%2520') + '&prefix-info=1';
        } else {
            var url = configURL.nginx.baseUrl + '/' + id + '/?delimiter=%2F&marker=&max-keys=50&prefix=&prefix-info=1';
            var params = {'prefix-info': 1, 'max-keys': 50};
        }
        return {
            request: {
                method: 'GET',
                url: url
            },
            response: {
                status: (element ? 200 : 401),
                data: (element ? mockFormatter.generateXml(element.resources.get.content, element, params) : null)
            }
        }
    };

    this.GET_mockS3_folder = function (folder) {
        var subdir = (folder.get.url == '') ? '%2F' : _.trimEnd(folder.get.url, '/').replace(/\//g, '%2F').replace(/ /g, '%2520');
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/?delimiter=%2F&prefix=' + subdir
            },
            response: {
                status: 200,
                data: mockFormatter.generateXml(folder.get.content, folder.xmlParams)
            }
        }
    };

    this.GET_mockS3_search_dir_without_subdir = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/?delimiter=%2F&depth=0&prefix=&search=dir'
            },
            response: {
                status: 200,
                data: mockFormatter.generateXml([
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir1"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir2"
                        }
                    }
                ], {})
            }
        }
    };

    this.GET_mockS3_search_dir_with_subdir = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/?delimiter=%2F&depth=&prefix=&search=dir'
            },
            response: {
                status: 200,
                data: mockFormatter.generateXml([
                    {
                        "Contents": {
                            "Key": "subdir2/subdir2-file1.txt",
                            "LastModified": "2015-07-15T06:38:33.287Z",
                            "Size": "7"
                        }
                    },
                    {
                        "Contents": {
                            "Key": "subdir1/subdir1-file1.txt",
                            "LastModified": "2015-07-15T06:38:33.287Z",
                            "Size": "8"
                        }
                    },
                    {
                        "Contents": {
                            "Key": "subdir1/subdir1-file2.txt",
                            "LastModified": "2015-07-15T06:38:33.287Z",
                            "Size": "15"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir1"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir2"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir2/dir_with_files/"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir1/subdir1-1/"
                        }
                    },
                    {
                        "CommonPrefixes": {
                            "Prefix": "subdir2/subdir1-2/"
                        }
                    }
                ], {})
            }
        }
    };

    this.GET_mockS3_search_unexisted_dir = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/?delimiter=%2F&depth=0&prefix=&search=unexisting'
            },
            response: {
                status: 200,
                data: mockFormatter.generateXml([], {})
            }
        }
    };

    /**
     * Get subdir1/subdir1-1/subdir1/unexisting-folder folder content
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockS3_root_subdir1_subdir1_1_unexisting_folder = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/?prefix=subdir1%2Fsubdir1-1%2Funexisting-folder&delimiter=%2F'
            },
            response: {
                status: 500,
                data: ''
            }
        }
    };

    /**
     * Get success file uploading - testFile.txt
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {msg: string}}}}
     */
    this.PUT_mockS3_testFile_txt = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/testFile.txt'
            },
            response: {
                status: 200,
                data: {
                    "msg": "File written"
                }
            }
        }
    };

    /**
     * Get success file uploading - subdir1/testRename.txt
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {msg: string}}}}
     */
    this.PUT_mockS3_subdir1_testRename_txt = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/subdir1/testRename.txt'
            },
            response: {
                status: 200,
                data: {
                    "msg": "File written"
                }
            }
        }
    };

    /**
     * Get delete file - subdir1/somefile.txt
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {msg: string}}}}
     */
    this.DELETE_mockS3_subdir1_somefile_txt = function () {
        return {
            request: {
                method: 'DELETE',
                url: configURL.nginx.baseUrl + '/' + containerId + '/subdir1/somefile.txt'
            },
            response: {
                status: 204,
                data: {
                    "msg": "File written"
                }
            }
        }
    };

    this.GET_mockS3_text_file_content = function (file, content) {
        return {
            request: {
                method: 'GET',
                url: configURL.nginx.baseUrl + '/' + containerId + '/' + file.get.url
            },
            response: {
                status: 200,
                data: content ? content : file.get.content
            }
        }
    };

    /**
     * Save content file - subdir1/somefile.txt
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {content: string}}}}
     */
    this.PUT_mockS3_save_text_content = function (file, content) {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/' + file.get.url
            },
            response: {
                status: 200,
                data: content
            }
        }
    };

    /**
     * Get success file uploading - subdir2/evenmore.txt
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {msg: string}}}}
     * @constructor
     */
    this.PUT_mockS3_subdir2_evenmore_txt = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/subdir2/evenmore.txt'
            },
            response: {
                status: 400,
                data: {
                    "msg": "File written"
                }
            }
        }
    };

    /**
     * Get success folder creating - test-dir
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.PUT_mockS3_test_dir = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/test-dir/'
            },
            response: {
                status: 201,
                data: mockFormatter.generateXml([])
            }
        }
    };

    /**
     * Get success folder creating - subdir1
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.PUT_mockS3_subdir1 = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/subdir1/'
            },
            response: {
                status: 500,
                data: mockFormatter.generateXml([])
            }
        }
    };

    /**
     * Get failed file uploading - File already exists
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {msg: string, directory: string, filename: string}}}}
     */
    this.PUT_mockS3_testFile_txt_already_exists = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/testFile.txt'
            },
            response: {
                status: 400,
                data: {
                    "msg": "File already exists",
                    "directory": "./resources/mockS3Server/server/",
                    "filename": "testFile.txt"
                }
            }
        }
    };

    /**
     * Get container statistic by request
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockBackend_container_statistics_by_request = function (request, statistic) {
        var url = configURL.api.baseUrl + '/' + request.refer;
        // set container 'id' param
        url += request.container ? ('/' + request.container.id) : (request.containerId ? ('/' + request.containerId) : '');
        // set 'projection' param
        url += request.projection ? ('/' + request.projection) : '';

        url += '/statistics?';
        // set 'filter' param
        url += request.filter ? ('filter=' + request.filter + '&') : '';
        request.filter = decodeURI(request.filter);
        // set 'from' query
        url += 'from=' + request.from;
        // set 'interval' query
        url += request.interval ? ('&interval=' + request.interval) : '';
        // set 'until' query
        url += request.until ? ('&until=' + request.until) : '';
        request.url = url;

        return {
            request: {
                method: 'GET',
                url: url
            },
            response: {
                status: 200,
                data: mockFormatter.generateOverviewStatistic(request, statistic)
            }
        }
    };

    /**
     * Get statistic for overview lower pane every 10s
     * @param {Object} request
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockBackend_overview_lowerpane_refresh_every_10s = function (request) {
        var id = _.defaultTo(request.containerId, containerId);
        var url = configURL.api.baseUrl + '/' + request.refer + '/' + id;
        // set 'projection' param
        url += request.projection ? ('/' + request.projection) : '';
        // set filter bandwidth
        url += '/statistics?filter=' + request.filter + '&from=-10s&interval=10s';
        request.url = url;
        request.filter = decodeURI(request.filter);
        request.interval = '10s';
        request.from = '-10s';
        request.until = null;

        return {
            request: {
                method: 'GET',
                url: url
            },
            response: {
                status: 200,
                data: mockFormatter.generateOverviewStatistic(request)
            }
        }
    };

    /**
     * Get statistic for overview lower pane
     * @param {Object} request (date in format - '2017-08-31 22:00')
     * @returns {{request: {method: string, url: string}, response: {status: number, data: Object}}}
     */
    this.GET_mockBackend_overview_lowerpane_statistic = function (request) {
        var id = _.defaultTo(request.containerId, containerId);
        var url = configURL.api.baseUrl + '/' + request.refer + '/' + id;
        // set 'projection' param
        url += request.projection ? ('/' + request.projection) : '';
        // set filter bandwidth
        url += '/statistics?filter=' + request.filter;
        // Set date (and time) from what statistic is needed (if don't set - period previous day will be used)
        var from = _.defaultTo(request.from, 'hour');
        var timeUntil = (request.until ? moment(request.until) : moment()).utc().format('HH:mm_YYYYMMDD');
        var timeFrom;
        // set factor's value according to time unit
        switch (from) {
            case 'hour':
                timeFrom = moment().utc().subtract(1, 'hours').format('HH:mm_YYYYMMDD');
                break;
            case 'day':
                timeFrom = moment().utc().subtract(1, 'days').format('HH:mm_YYYYMMDD');
                break;
            case 'week':
                timeFrom = moment().utc().subtract(7, 'days').format('HH:mm_YYYYMMDD');
                break;
            case 'month':
                timeFrom = moment().utc().subtract(1, 'months').format('HH:mm_YYYYMMDD');
                break;
            case 'year':
                timeFrom = moment().utc().subtract(1, 'years').format('HH:mm_YYYYMMDD');
                break;
            default:
                timeFrom = moment(from).utc().format('HH:mm_YYYYMMDD');
                break;
        };

        // Set interval amount
        var rangeLengthInSeconds = moment(timeUntil, 'HH:mm_YYYYMMDD').diff(moment(timeFrom, 'HH:mm_YYYYMMDD'), 'seconds');
        var intervalSeconds = Math.floor(rangeLengthInSeconds / 300);
        var intervalTens = Math.round(intervalSeconds / 10) * 10;
        var interval = Math.max(10, intervalTens) + 's';

        url += '&from=' + timeFrom + '&interval=' + interval + (request.until ? ('&until=' + timeUntil) : '');

        return {
            request: {
                method: 'GET',
                url: url
            },
            response: {
                status: 200,
                data: mockFormatter.generateOverviewStatistic({
                    containerId: id,
                    refer: request.refer,
                    from: timeFrom,
                    interval: interval,
                    until: request.until ? timeUntil : null,
                    url: url,
                    filter: decodeURI(request.filter)
                })
            }
        }
    };

    /**
     * Update resource mock
     * @param {string} resourceId
     * @param {Object} updatingData - object that contains data for updating mock
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {}}}}
     * @constructor
     */
    this.PUT_mockBackend_API_resources = function (resourceID, updatingData) {
        var resource = JSON.parse(JSON.stringify(mockFormatter.getMockData(updatingData.resources).data.filter(function (resources) {
            return resources.id === resourceID;
        })[0]));
        var outdateResource = _.cloneDeep(resource);

        _.forEach(updatingData, function (value, key) {
            if (_.has(resource, key)) {
                resource[key] = value;
            }
            if (_.has(resource.attributes, key)) {
                resource.attributes[key] = value;
            }
            if (_.has(resource.relationships, key)) {
                resource.relationships[key] = value;
            }
        });

        if (updatingData.resources === 'user-groups') {
            mockFormatter.updateBackRelationships(resource, outdateResource);
        }

        return {
            request: {
                method: 'PUT',
                url: configURL.api.baseUrl + '/' + updatingData.resources.replace(/-/g, '_') + '/' + resourceID
            },
            response: {
                status: 200,
                data: {data: resource}
            }
        }
    };

    /**
     * Overview sessions mock
     * @returns {Object}
     */
    this.GET_mockBackend_overview_sessions = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + containerId + '/interfaces/abff0d45-8218-40a0-94d4-b84617a4309c?include=details,details.user'
            },
            response: {
                status: 200,
                data: mockFormatter.generateOverviewSessions('abff0d45-8218-40a0-94d4-b84617a4309c',
                    [
                        'details',
                        'details.user'
                    ]
                )
            }
        }
    };

    /**
     * Interfaces mock
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {success: boolean, results: *[]}}}}
     */
    this.GET_mockBackend_data_policy = function (id) {
        id = _.defaultTo(id, containerId);
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + id + '?fields%5Bcontainer%5D=data_policy_layers&fields%5Buser_group%5D=name' +
                '&include=data_policy_layers,data_policy_layers.user,data_policy_layers.rules,data_policy_layers.rules.match_datetime_range,data_policy_layers.rules.match_networks,' +
                'data_policy_layers.rules.match_users,data_policy_layers.rules.match_user_groups,data_policy_layers.rules.match_categories,' +
                'data_policy_layers.rules.access_override_user,data_policy_layers.rules.functions,data_policy_layers.groups,data_policy_layers.groups.rules,data_policy_layers.groups.rules.match_datetime_range,' +
                'data_policy_layers.groups.rules.match_networks,data_policy_layers.groups.rules.match_users,data_policy_layers.groups.rules.match_user_groups,' +
                'data_policy_layers.groups.rules.match_categories,data_policy_layers.groups.rules.access_override_user,data_policy_layers.groups.rules.functions'
            },
            response: {
                status: 200,
                data: mockFormatter.generateContainer(id,
                    {container: 'data_policy_layers', user_group: 'name'},
                    ['data_policy_layers',
                        'data_policy_layers.user',
                        'data_policy_layers.rules',
                        'data_policy_layers.rules.match_datetime_range',
                        'data_policy_layers.rules.match_networks',
                        'data_policy_layers.rules.match_users',
                        'data_policy_layers.rules.match_user_groups',
                        'data_policy_layers.rules.match_categories',
                        'data_policy_layers.rules.access_override_user',
                        'data_policy_layers.rules.functions',
                        'data_policy_layers.groups',
                        'data_policy_layers.groups.rules',
                        'data_policy_layers.groups.rules.match_datetime_range',
                        'data_policy_layers.groups.rules.match_networks',
                        'data_policy_layers.groups.rules.match_users',
                        'data_policy_layers.groups.rules.match_user_groups',
                        'data_policy_layers.groups.rules.match_categories',
                        'data_policy_layers.groups.rules.access_override_user',
                        'data_policy_layers.groups.rules.functions']
                )
            }
        }
    };

    /**
     * Post Interfaces mock (on interface pane)
     * @param type - interface type
     * @param alias - interface alias
     * @param networkId - network id
     * @param containerId - container id
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {success: boolean, results: {type: string, id: string, attributes: {_type: *, name: *, sessions: number, traffic: number, active: boolean, created_at: string}, relationships: {container: {data: {type: string, id: string}}}}}}}}
     * @constructor
     */
    this.POST_mockBackend_overview_interfaces = function (type, alias, networkId, containerId) {
        var includedContainer = mockFormatter.getResourceMock({id: containerId}, {}, 'container', undefined);
        var includedNetwork = mockFormatter.getResourceMock({id: networkId}, {}, 'network', undefined);

        var responseData = {
            "data": {
                "type": "interface",
                "id": "8c097455-ebb0-47cd-9e31-bec1f95656e8",
                "attributes": {
                    "kind": type,
                    "alias": alias,
                    "created_at": "2015-11-05T14:33:08.066Z"
                },
                "relationships": {
                    "container": {
                        "data": {
                            "type": "container",
                            "id": containerId
                        }
                    },
                    "network": {
                        "data": {
                            "type": "network",
                            "id": networkId
                        }
                    }
                }
            },
            "included": [
                includedContainer,
                includedNetwork
            ]
        };

        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/interfaces'
            },
            response: {
                status: 201,
                "data": responseData
            }
        }
    };

    this.OPTIONS_mockBackend_overview_interfaces = function () {
        return {
            request: {
                method: 'OPTIONS',
                url: configURL.api.baseUrl + '/containers/' + containerId + '/interface'
            },
            response: {
                status: 204,
                headers: {
                    "Access-Control-Allow-Credentials": true,
                    "Access-Control-Allow-Headers": "accept, content-type",
                    "Access-Control-Allow-Methods": "GET,HEAD,PUT,PATCH,POST,DELETE",
                    "Access-Control-Allow-Origin": "http://127.0.0.1:8000",
                    "Connection": "keep-alive",
                    "Date": "Thu, 05 Nov 2015 14:33:08 GMT",
                    "Vary": "Origin",
                    "X-Powered-By": "Express"
                }
            }
        }
    };

    /**
     * events mock
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {success: boolean, results: *[]}}}}
     */
    this.GET_mockBackend_overview_events = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + containerId + '?fields%5Bcontainer%5D=id&include=events'
            },
            response: {
                status: 200,
                data: mockFormatter.generateContainer(containerId,
                    {container: 'id'},
                    ['events']
                )
            }
        }
    };

    this.POST_mockBackend_Accept_Button = function (id, description, type, severity, state) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/containers/' + containerId + '/events/' + id
            },
            response: {
                status: 200,
                data: {
                    "data": {
                        "type": "event",
                        "id": id,
                        "attributes": {
                            "description": description,
                            "_type": type,
                            "severity": severity,
                            "state": "acknowledged",
                            "created_at": "2015-10-04T17:07:48.509Z"
                        }
                    }
                }
            }
        }
    };

    /**
     * Tasks mock
     * @returns {{request: {method: string, url: string}, response: {status: number, data: {success: boolean, results: *[]}}}}
     */
    this.GET_mockBackend_overview_tasks = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/containers/' + containerId + '?fields%5Bcontainer%5D=id&include=jobs'
            },
            response: {
                status: 200,
                data: mockFormatter.generateContainer(containerId,
                    {container: 'id'},
                    ['jobs']
                )
            }
        }
    };

    this.POST_mockBackend_API_networks = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/networks'
            },
            response: {
                status: 200,
                data:  {
                    "data": mockFormatter.generateNewNetwork(data)
                }
            }
        }
    };

    this.POST_mockBackend_API_networks_duplicate = function (id) {
        var networksList = mockFormatter.getMockData('networks').data;
        var newNetwork;

        for (var item in networksList) {
            if (networksList[item].id === id) {
                newNetwork = _.cloneDeep(networksList[item])
            }
        }

        newNetwork.attributes.name += ' - Copy_123';
        newNetwork.id += '123abc';

        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/networks'
            },
            response: {
                status: 200,
                data: {
                    "data": newNetwork
                }
            }
        }
    };

    this.GET_mockBackend_containers_info = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.push.baseUrl + '/info'
            },
            response: {
                status: 200,
                data: {"websocket": true, "origins": ["*:*"], "cookie_needed": false, "entropy": 326954968}
            }
        }
    };

    this.POST_mockBackend_password_resets = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/password_resets'
            },
            response: {
                status: 201,
                data: mockFormatter.generatePasswordReset(data)
            }
        }
    };

    this.GET_mockBackend_API_password_resets = function (resetPasswordID) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/password_resets/' + resetPasswordID
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({'id': resetPasswordID}, {}, 'password_reset', undefined)
            }
        }
    };

    this.POST_mockBackend_wrong_password_resets = function () {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/password_resets'
            },
            response: {
                status: 404,
                data: {"errors": [{"detail": "Not found", "status": 404}]}
            }
        }
    };

    this.PUT_mockBackend_password_resets = function () {
        return {
            request: {
                method: 'PUT',
                url: configURL.api.baseUrl + '/password_resets/123123'
            },
            response: {
                status: 400,
                data: {"results": {"username": "iguaz.io user", "id": "9a9530ac-91f2-11e5-8994-feff819cdc9f"}}
            }
        }
    };

    this.GET_mockBackend_API_critical_alerts = function (params) {
        var query = _.extend(params, {
            sort: '-created_at'
        });
        return {
            request: {
                method: 'GET',
                url: '/' + configURL.api.baseUrl.replace(/\\/, '\/') + '\/alerts\\?filter%5Bseverity%5D=critical&filter%5Bupdated_at%5D=%5B\\$ge%5D(\\d{4})-(\\d{2})-(\\d{2})T((\\d{2}):(\\d{2}):(\\d{2}))\\.(\\d{3})Z&filter%5Buser_status%5D=none&sort=-created_at/'
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({}, query, 'alert', {
                    "attributes": {
                        "severity": "critical"
                    }
                })
            }
        }
    };

    this.POST_mockBackend_alert_resolutions = function (alert) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/alert_resolutions'
            },
            response: {
                status: 200,
                data: {
                    "included": [],
                    "data": {
                        "type": "alert",
                        "id": alert.id,
                        "attributes": {
                            "user_status": alert.status,
                            "escalation_status": "stopped"
                        }, "relationships": {
                            "assignee": {
                                "data": {
                                    "type": "user", "id": "66b6fd2e-be70-477c-8226-7991757c0d79"
                                }
                            }
                        }
                    }
                }
            }
        }
    };

    this.POST_mockBackend_alert_acknowledgements = function (alert) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/alert_acknowledgements'
            },
            response: {
                status: 200,
                data: {
                    "included": [],
                    "data": {
                        "type": "alert",
                        "id": alert.id,
                        "attributes": {
                            "user_status": alert.status,
                            "escalation_status": "stopped"
                        }, "relationships": {
                            "assignee": {
                                "data": {
                                    "type": "user", "id": "66b6fd2e-be70-477c-8226-7991757c0d79"
                                }
                            }
                        }
                    }
                }
            }
        }
    };

    this.GET_mockBackend_API_cluster_id_nodes = function (cluster_id, request) {
        var requestUrlString = _.join(request.include, ',') + '&';
        request.customFilter = {
            "relationships": {
                "cluster": {
                    "data": {
                        "type": "cluster",
                        "id": cluster_id
                    }
                }
            }
        };

        if (_.has(request, 'pagination')) {
            requestUrlString += 'pagination=' + request.pagination
        }

        if (_.has(request, 'page')) {
            requestUrlString += 'page%5Bnumber%5D=' + request.page + '&'
        }

        if (_.has(request, 'pageOf')) {
            requestUrlString += 'page%5Bof%5D=' + request.pageOf + '&'
        }

        if (_.has(request, 'perPage')) {
            requestUrlString += 'page%5Bsize%5D=' + request.perPage
        }

        if (_.has(request, 'sort')) {
            requestUrlString += '&sort=' + request.sort
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/clusters/' + cluster_id + '/nodes?include=' + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList(request)
            }
        }
    };

    this.GET_mockBackend_API_storage_pools_id_containers = function (storage_pool_id, request) {
        var requestUrlString = '';

        if (_.has(request, 'pagination')) {
            requestUrlString += 'pagination=' + request.pagination
        }

        if (_.has(request, 'page')) {
            requestUrlString += 'page%5Bnumber%5D=' + request.page + '&'
        }

        if (_.has(request, 'pageOf')) {
            requestUrlString += 'page%5Bof%5D=' + request.pageOf + '&'
        }

        if (_.has(request, 'perPage')) {
            requestUrlString += 'page%5Bsize%5D=' + request.perPage
        }

        if (_.has(request, 'sort')) {
            requestUrlString += '&sort=' + request.sort
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/storage_pools/' + storage_pool_id + '/containers?include=owner&' + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList(request)
            }
        }
    };

    this.PUT_mockBackend_API_cluster_nodes = function (clusterID, nodeID, updatingData) {
        var node = JSON.parse(JSON.stringify(mockFormatter.getMockData('nodes').data.filter(function (nodes) {
            return nodes.id === nodeID;
        })[0]));

        _.forEach(updatingData, function (value, key) {
            if (_.has(node, key)) {
                node[key] = value;
            }
            if (_.has(node.attributes, key)) {
                node.attributes[key] = value;
            }
            if (_.has(node.relationships, key)) {
                node.relationships[key] = value;
            }
        });

        return {
            request: {
                method: 'PUT',
                url: configURL.api.baseUrl + '/clusters/' + clusterID + '/nodes/' + nodeID
            },
            response: {
                status: 200,
                data: node
            }
        }
    };

    this.GET_mockBackend_clusters_id_include_owner = function (cluster_id) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/clusters/' + cluster_id + '?include=owner'
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({id: cluster_id}, {include: ['owner']}, 'cluster')
            }
        }
    };

    this.POST_mockBackend_API_escalation_policy_filters = function (policyID) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/escalation_policy_filters'
            },
            response: {
                status: 201,
                data: {
                    "data": {
                        "type": "escalation_policy_filter",
                        "attributes": {
                            "tags": [],
                            "severity": "",
                            "created_at": "2016-09-12T07 :33:33.892Z"
                        },
                        "relationships": {
                            "escalation_policies": {
                                "data": [
                                    {
                                        "type": "escalation_policy",
                                        "id": policyID
                                    }
                                ]
                            }
                        },
                        "id": "7ff0b634-4f0d-4a53-9166-19ddeacb80c5"
                    }
                }
            }
        }
    };

    this.GET_mockBackend_API_users_include_groups = function (query) {
        query.include = ['user_groups', 'primary_group'];
        var requestUrlString = '/users?include=' + query.include.join(',');

        if (!query.sort) {
            query.sort = 'first_name,last_name'
        }

        if (_.has(query, 'page')) {
            requestUrlString += '&page%5Bnumber%5D=' + query.page.number + '&page%5Bsize%5D=' + query.page.size + '&sort=' + query.sort;
        }

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({}, query, 'user')
            }
        }
    };

    this.GET_mockBackend_API_user_group_records = function (userGroupId) {
        var requestUrlString = '';
        var customFilter;

        if (!_.isUndefined(userGroupId)) {
            requestUrlString = '/user_groups/' + userGroupId;
            customFilter = function (user) {
                return _.some(_.get(user, 'relationships.user_groups.data', []), ['id', userGroupId]);
            };
        }

        requestUrlString += '/users?count=records&page%5Bsize%5D=0';

        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({}, {countRecords: 'records'}, 'user', customFilter)
            }
        }
    };

    this.GET_mockBackend_API_user_group_users = function (userGroupId, query) {
        query.resources = 'users';
        var requestUrlString = '/user_groups/' + userGroupId + '/' + query.resources + '?';

        if (_.has(query, 'filter')) {
            _.forEach(query.filter, function (key, value) {
                requestUrlString += 'filter%5B' + value + '%5D=' + encodeURI(key) + '&';
            });
        }

        if (_.has(query, 'page')) {
            requestUrlString += 'page%5Bnumber%5D=' + query.page + '&';
        }

        if (_.has(query, 'perPage')) {
            requestUrlString += 'page%5Bsize%5D=' + query.perPage;
        }

        if (query.sort) {
            requestUrlString += '&sort=' + query.sort;
        }

        if (!_.isUndefined(userGroupId)) {
            query.customFilter = function (user) {
                return _.some(_.get(user, 'relationships.user_groups.data', []), ['id', userGroupId]);
            };
        }
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + requestUrlString
            },
            response: {
                status: 200,
                data: mockFormatter.generateResourcesList(query)
            }
        }
    };

    this.GET_mockBackend_users_id = function (userID) {
        userID = userID ? userID : defaultUserId;
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/users/' + userID
            },
            response: {
                status: 200,
                data: mockFormatter.getResourceMock({id: userID}, {}, 'user', undefined)
            }
        }
    };

    this.POST_mockBackend_API_users = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/users'
            },
            response: {
                status: 200,
                data: mockFormatter.generateNewUser(data)
            }
        }
    };

    this.POST_mockBackend_API_tenants = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/tenants'
            },
            response: {
                status: 200,
                data: mockFormatter.generateNewTenant(data)
            }
        }
    };

    this.POST_mockBackend_conflict = function (data) {
        var title = {
            405: "Method Not Allowed",
            409: "Conflict"
        };
        var detail = {
            405: "Resource limit reached. Cannot create more records",
            409: "Field uniqueness violation: " + data.field
        };
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/' + data.resources
            },
            response: {
                status: data.status,
                data: {
                    "errors": [
                        {
                            "status": data.status,
                            "title": title[data.status],
                            "detail": detail[data.status]
                        }
                    ]
                }
            }
        }
    };

    this.POST_mockBackend_API_user_groups = function (request) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/user_groups'
            },
            response: {
                status: 201,
                data: mockFormatter.generateUserGroup(request)
            }
        }
    };

    this.PUT_mockBackend_API_user_groups_existed = function (groupID) {
        return {
            request: {
                method: 'PUT',
                url: configURL.api.baseUrl + '/user_groups/' + groupID
            },
            response: {
                status: 409,
                data: {"errors": [{"status": 409, "title": "Conflict", "detail": "Field uniqueness violation: name"}]}
            }
        }
    };

    this.PUT_mockBackend_API_user_groups_forbidden = function (groupID) {
        return {
            request: {
                method: 'PUT',
                url: configURL.api.baseUrl + '/user_groups/' + groupID
            },
            response: {
                status: 403,
                data: {"errors": [{"status": 403}]}
            }
        }
    };

    this.GET_mockBackend_API_browser_tags = function () {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/browser_tags'
            },
            response: {
                status: 200,
                data: {
                    "data": [
                        {
                            "type": "browser_tag",
                            "id": "1-ff9298f6-df92-480b-aeb1-6bfcfa450eae",
                            "attributes": {
                                "name": "Data"
                            }
                        },
                        {
                            "type": "browser_tag",
                            "id": "2-ff9298f6-df92-480b-aeb1-6bfcfa450eae",
                            "attributes": {
                                "name": "Design"
                            }
                        },
                        {
                            "type": "browser_tag",
                            "id": "3-ff9298f6-df92-480b-aeb1-6bfcfa450eae",
                            "attributes": {
                                "name": "Music"
                            }
                        },
                        {
                            "type": "browser_tag",
                            "id": "4-ff9298f6-df92-480b-aeb1-6bfcfa450eae",
                            "attributes": {
                                "name": "UI"
                            }
                        }]
                }
            }
        }
    };

    this.POST_mockBackend_API_browser_tags = function (name) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/browser_tags'
            },
            response: {
                status: 201,
                data: {"data": {"type": "browser_tag", "attributes": {"name": name}, "id": "398a9bed-9cda-a973-9784-9d64db95713f"}}
            }
        }
    };

    this.POST_mockBackend_API_transactions = function (status) {
        var responseStatus = _.defaultTo(status, 200)
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/transactions'
            },
            response: {
                status: responseStatus,
                data: {}
            }
        }
    };

    this.POST_mockBackend_API_storage_pools = function (data) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/storage_pools'
            },
            response: {
                status: 200,
                data: mockFormatter.generateNewStoragePool(data)
            }
        }
    };

    this.PUT_mockS3_folder_get_item = function (folder, attributesToGet, pagination, filterExpression, id) {
        var pagination = pagination || {limit: 50, marker: ""};
        id = _.defaultTo(id, containerId);
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + id + '/' + encodeURI(folder.put.url)
            },
            response: {
                status: 200,
                data: mockFormatter.getItemsHandler(folder, attributesToGet, pagination, filterExpression)
            }
        }
    };

    this.PUT_mockS3_file_get_item = function (file, attributesToGet) {
        return {
            request: {
                method: 'PUT',
                url: configURL.nginx.baseUrl + '/' + containerId + '/' + file.put.url
            },
            response: {
                status: 200,
                data: mockFormatter.getItemHandler(file, attributesToGet)
            }
        }
    };

    this.PUT_mockS3_folder_put_item = function (folder, putAttributes) {
        var folder = (folder.get.url.lastIndexOf('/') > -1) ? folder.get.url.substring(0, folder.get.url.lastIndexOf('/')) : folder.get.url;
        var url = configURL.nginx.baseUrl + '/' + containerId + '/' + folder;
        if (!_.endsWith(url, '/')) {
            url += '/';
        }
        return {
            request: {
                method: 'PUT',
                url: url
            },
            response: {
                status: 204,
                data: mockFormatter.putItemHandler(folder, putAttributes)
            }
        }
    };

    this.DELETE_mockS3_folder_put_item = function (folder, error) {
        var folder = (folder.get.url.lastIndexOf('/') > -1) ? folder.get.url.substring(0, folder.get.url.lastIndexOf('/')) : folder.get.url;
        var url = configURL.nginx.baseUrl + '/' + containerId + '/' + folder;
        if (!_.endsWith(url, '/')) {
            url += '/';
        }
        return {
            request: {
                method: 'DELETE',
                url: url
            },
            response: {
                status: error ? 409 : 204,
                data: mockFormatter.putItemHandler(folder)
            }
        }
    };

    this.DELETE_mockS3_file_put_item = function (file) {
        var file = (file.get.url.lastIndexOf('/') > -1) ? file.get.url.substring(0, file.get.url.lastIndexOf('/')) : file.get.url;
        var url = configURL.nginx.baseUrl + '/' + containerId + '/' + file;
        return {
            request: {
                method: 'DELETE',
                url: url
            },
            response: {
                status: 204,
                data: mockFormatter.putItemHandler(file)
            }
        }
    };

    this.GET_mockServer_Image = function (file) {
        var url = file.get.url;
        var requestURL = '/' + containerId + '/' + url;
        return {
            method: 'get',
            url: requestURL,
            handler: function (req, response) {
                var path = e2e_root + '/' + url;

                var headersList = {
                    'Content-Type': 'image',
                    'Content-Length': 45546,
                    'Content-Disposition': 'inline'
                };
                var data = fs.readFileSync(path);

                response.set(headersList);
                response.send(data);
            }
        }
    };

    this.GET_mockServer_container_XML = function (folder) {
        return {
            method: 'get',
            url: '/' + containerId + '/',
            handler: function (req, res) {
                var data = mockFormatter.generateXml(folder.get.content, folder.xmlParams);

                res.type('xml');
                res.send(data);
            }
        }
    };

    this.GET_mockBackend_API_tenants_job = function (request) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/jobs?filter%5Bkind%5D=idp.synchronize.hierarchy&filter%5Btenant%5D=' + request.tenantId + '&page%5Bsize%5D=1&sort=' + (request.sort ? request.sort : '-created_at')
            },
            response: {
                status: 200,
                data: {
                    "data": [],
                    "meta": {
                        "total_pages": 0
                    }
                }
            }
        }
    };

    this.POST_mockBackend_API_tenants_syncs = function (tenantId, status) {
        return {
            request: {
                method: 'POST',
                url: configURL.api.baseUrl + '/tenants/' + tenantId + '/idp_syncs'
            },
            response: {
                status: status,
                data: {
                    "data": {
                        "type": "job",
                        "id": "6ed5ed39-d234-4b14-a1a2-e57a47bcdcc8"
                    }
                }
            }
        }
    };

    this.GET_mockBackend_API_tenants_syncs = function (tenantId, conflictStatus) {
        return {
            request: {
                method: 'GET',
                url: configURL.api.baseUrl + '/jobs/6ed5ed39-d234-4b14-a1a2-e57a47bcdcc8'
            },
            response: {
                status: 200,
                data: mockFormatter.tenantHandler(tenantId, conflictStatus)
            }
        }
    };
};

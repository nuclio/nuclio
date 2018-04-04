describe('SearchHelperService: ', function () {
    var SearchHelperService;
    var data;
    var searchKeys = [
        'attr.name',
        'rels.match_interfaces',
        'rels.match_networks',
        'rels.match_users',
        'rels.match_user_groups',
        'rels.match_categories',
        'attr.match_tags',
        'attr.match_objects',
        'rels.access_override_user'
    ];
    var searchStates = {
        searchNotFound: false,
        searchInProgress: false
    };
    var isSearchHierarchically = true;
    var ruleType = 'data_policy_rule';

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_SearchHelperService_) {
            SearchHelperService = _SearchHelperService_;
        });

        data = {
            type: 'root',
            ui: {
                children: [
                    {
                        type: 'data_policy_layer',
                        attr: {
                            name: 'Layer 1'
                        },
                        ui: {
                            isShowed: true,
                            children: [
                                {
                                    type: 'data_policy_rule',
                                    attr: {
                                        name: 'Rule First',
                                        created_at: new Date('2015-04-10T00:00:00Z'),
                                        expires_on: new Date('2016-05-02T00:00:00Z'),
                                        access_permissions_ops_full: true,
                                        access_permissions_mode: 'allow',
                                        access_limits_bandwidth_read: 157286400,
                                        access_service_level_priority: 'high',
                                        match_tags: [
                                            {
                                                title: 'The Tag',
                                                type: 'tag'
                                            }
                                        ],
                                        match_objects: [],
                                        match_interfaces: ['All First']
                                    },
                                    rels: {
                                        match_networks: [
                                            {
                                                type: 'network',
                                                attr: {
                                                    name: 'backup (192.169.1.1/7) First'
                                                }
                                            }
                                        ],
                                        match_users: [
                                            {
                                                type: 'user',
                                                attr: {
                                                    name: 'Joe_md First'
                                                }
                                            }
                                        ],
                                        match_categories: [
                                            {
                                                attr: {
                                                    name: 'NASA Docs',
                                                    kind: 'documents'
                                                }
                                            }
                                        ],
                                        functions: []
                                    },
                                    ui: {
                                        isShowed: true
                                    }
                                }
                            ]
                        }
                    },
                    {
                        type: 'data_policy_layer',
                        attr: {
                            name: 'Layer 2'
                        },
                        ui: {
                            isFitQuery: true,
                            children: [
                                {
                                    type: 'data_policy_rule',
                                    attr: {
                                        name: 'Rule Second Test 1',
                                        access_permissions_ops_full: true,
                                        access_permissions_mode: 'allow',
                                        access_limits_bandwidth_read: 157286400,
                                        access_service_level_priority: 'high',
                                        match_tags: [
                                            {
                                                title: 'The Tag',
                                                type: 'tag'
                                            }
                                        ],
                                        match_objects: [],
                                        match_interfaces: ['All']
                                    },
                                    rels: {
                                        match_networks: [
                                            {
                                                type: 'network',
                                                attr: {
                                                    name: 'backup (192.169.1.1/7) Second'
                                                }
                                            }
                                        ],
                                        match_users: [
                                            {
                                                type: 'user',
                                                attr: {
                                                    name: 'Joe_md Second'
                                                }
                                            }
                                        ],
                                        match_categories: [
                                            {
                                                attr: {
                                                    name: 'NASA Docs',
                                                    kind: 'documents'
                                                }
                                            }
                                        ],
                                        functions: []
                                    },
                                    ui: {
                                        isShowed: true,
                                        isFitQuery: true
                                    }
                                },
                                {
                                    type: 'data_policy_group',
                                    attr: {
                                        name: 'Mega Group'
                                    },
                                    ui: {
                                        isFitQuery: true,
                                        children: [
                                            {
                                                type: 'data_policy_rule',
                                                attr: {
                                                    name: 'Rule Third Test 1',
                                                    created_at: new Date('2015-04-10T00:00:00Z'),
                                                    expires_on: new Date('2016-05-02T00:00:00Z'),
                                                    access_permissions_ops_full: true,
                                                    access_permissions_mode: 'allow',
                                                    access_limits_bandwidth_read: 157286400,
                                                    access_service_level_priority: 'high',
                                                    match_tags: [
                                                        {
                                                            title: 'The Tag',
                                                            type: 'tag'
                                                        }
                                                    ],
                                                    match_objects: [],
                                                    match_interfaces: ['All']
                                                },
                                                rels: {
                                                    match_networks: [
                                                        {
                                                            type: 'network',
                                                            attr: {
                                                                name: 'backup (192.169.1.1/7) Third'
                                                            }
                                                        }
                                                    ],
                                                    match_users: [
                                                        {
                                                            type: 'user',
                                                            attr: {
                                                                name: 'Joe_md Third'
                                                            }
                                                        }
                                                    ],
                                                    match_categories: [
                                                        {
                                                            attr: {
                                                                name: 'NASA Docs',
                                                                kind: 'documents'
                                                            }
                                                        }
                                                    ],
                                                    functions: []
                                                },
                                                ui: {
                                                    isShowed: true,
                                                    isFitQuery: true
                                                }
                                            }
                                        ]
                                    }
                                }
                            ]
                        }
                    }
                ]
            }
        };

        // Add proper links to `parents`
        data.ui.children[0].ui.children[0].ui.parent = data.ui.children[0];
        data.ui.children[1].ui.children[0].ui.parent = data.ui.children[1];
        data.ui.children[1].ui.children[1].ui.parent = data.ui.children[1];
        data.ui.children[1].ui.children[1].ui.children[0].ui.parent = data.ui.children[1].ui.children[1];
    });

    afterEach(function () {
        SearchHelperService = null;
        data = null;
    });

    describe('makeSearch(): ', function () {
        it('should perform search through all rules in data, including nested ones into a group', function () {
            var searchQuery = 'test 1';
            var topLevelRule = data.ui.children[1].ui.children[0];
            var nestedToAGroupRule = data.ui.children[1].ui.children[1].ui.children[0];
            var someNotRelevantRule = data.ui.children[0].ui.children[0];
            SearchHelperService.makeSearch(searchQuery, data, searchKeys, isSearchHierarchically, ruleType, searchStates);
            expect(topLevelRule.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.isFitQuery).toBe(true);
            expect(someNotRelevantRule.ui.isFitQuery).toBe(false);
            // Check search status
            expect(searchStates.searchInProgress).toBeTruthy();
        });

        it('should make related parents visible for every relevant rule', function () {
            var searchQuery = 'test 1';
            var topLevelRule = data.ui.children[1].ui.children[0];
            var nestedToAGroupRule = data.ui.children[1].ui.children[1].ui.children[0];
            var someNotRelevantRule = data.ui.children[0].ui.children[0];

            SearchHelperService.makeSearch(searchQuery, data, searchKeys, isSearchHierarchically, ruleType, searchStates);
            expect(topLevelRule.ui.parent.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.parent.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.parent.ui.parent.ui.isFitQuery).toBe(true);
            expect(someNotRelevantRule.ui.parent.ui.isFitQuery).toBe(false);
            // Check search status
            expect(searchStates.searchInProgress).toBeTruthy();
        });

        it('should show all rules and all their related parents if search query is an empty string', function () {
            var searchQuery = '';
            var topLevelRule = data.ui.children[1].ui.children[0];
            var nestedToAGroupRule = data.ui.children[1].ui.children[1].ui.children[0];
            var someNotRelevantRule = data.ui.children[0].ui.children[0];

            SearchHelperService.makeSearch(searchQuery, data, searchKeys, isSearchHierarchically, ruleType, searchStates);

            expect(searchStates.searchNotFound).toBeFalsy();

            expect(topLevelRule.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.isFitQuery).toBe(true);
            expect(someNotRelevantRule.ui.isFitQuery).toBe(true);

            expect(topLevelRule.ui.parent.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.parent.ui.isFitQuery).toBe(true);
            expect(nestedToAGroupRule.ui.parent.ui.parent.ui.isFitQuery).toBe(true);
            expect(someNotRelevantRule.ui.parent.ui.isFitQuery).toBe(true);
            // Check search status
            expect(searchStates.searchInProgress).toBeFalsy();
        });
    });
});

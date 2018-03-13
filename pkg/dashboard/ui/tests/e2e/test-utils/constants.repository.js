module.exports = function () {
    var baseURL = 'http://127.0.0.1:8000';
    var containerID = 748911;
    var storagePoolID = '5cc92c08-8dcf-41b0-9816-84f9127d334f';
    var clusterID = '362b4461-840c-4662-9065-6a81050cc19b';

    this.sessionId = '9a9530ac-91f2-11e5-8994-feff819cdc9f';
    this.ipAdress = '127.0.0.1';

    this.loginURL = baseURL + '/login';
    this.loginTenantURL = baseURL + '/login?tenant';
    this.resetPasswordURL = '/login/reset-password/';
    this.forgotPasswordURL = baseURL + '/login/forgot-password';
    this.analyticsURL = baseURL + '/containers/' + containerID + '/analytics';
    this.dataAccessPolicyURL = baseURL + '/containers/' + containerID + '/data-access-policy';
    this.browseURL = baseURL + '/containers/' + containerID + '/browse';
    this.overviewURL = baseURL + '/containers/' + containerID + '/overview';
    this.containersURL = function (params) {
        if(params.id) {
            return baseURL + '/containers?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/containers?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.lifecycleURL = baseURL + '/containers/' + containerID + '/data-lifecycle';
    this.eventsAlertsURL = function (params) {
        if(params.id) {
            return baseURL + '/events/alerts?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/events/alerts?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.eventLogURL = function (params) {
        if(params.id) {
            return baseURL + '/events/event-log?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/events/event-log?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.eventsEscalationURL = function (params) {
        if(params.id) {
            return baseURL + '/events/escalation?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/events/escalation?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.tasksURL = function (params) {
        if(params.id) {
            return baseURL + '/events/tasks?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/events/tasks?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.usersURL = function (params) {
        if(params.id) {
            return baseURL + '/identity/users?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/identity/users?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.groupsURL = function (params) {
        if(params.id) {
            return baseURL + '/identity/groups?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/identity/groups?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.tenantsURL = function (params) {
        if(params.id) {
            return baseURL + '/tenants?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/tenants?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.networksURL = function (params) {
        if(params.id) {
            return baseURL + '/networks?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/networks?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.storagePoolsURL = function (params) {
        if(params.id) {
            return baseURL + '/storage-pools?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/storage-pools?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.storagePoolsContainersURL = function (params) {
        if(params.id) {
            return baseURL + '/storage-pools/' + storagePoolID + '/containers?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/storage-pools/' + storagePoolID + '/containers?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.storagePoolsDevicesURL = function (params) {
        return baseURL + '/storage-pools/' + storagePoolID + '/devices?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.storagePoolsOverviewURL = baseURL + '/storage-pools/' + storagePoolID + '/overview';
    this.clustersURL = function (params) {
        if(params.id) {
            return baseURL + '/clusters?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/clusters?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
    this.nodesURL = function (params) {
        if(params.id) {
            return baseURL + '/clusters/' + clusterID + '/nodes?id=' + params.id + '&pageSize=' + params.pageSize;
        } else
        return baseURL + '/clusters/' + clusterID + '/nodes?pageSize=' + params.pageSize + '&pageNumber=' + (params.pageNumber + 1);
    };
};
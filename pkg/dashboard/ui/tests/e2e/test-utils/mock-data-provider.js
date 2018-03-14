module.exports = function () {
    var getEntitiesArray = function (type) {
        return require('./mock-data.repository/' + type + '.json');
    };

    /**
     * Get parsed data entities
     * @param {string} type
     * @returns {{}}
     */
    var getEntities = function (type) {
        var data = getEntitiesArray(type);

        var parsed = {};
        _.forEach(data, function (entity) {
            parsed[entity.id] = entity;
        });

        return parsed;
    };

    /**
     * Generate Relationships object
     * @param {Object} filterData
     * @returns {{relationships: {}}}
     */
    var generateRelationshipsData = function (filterData) {
        var key = Object.keys(filterData)[0];

        var rel = {
            "relationships": {}
        };
        rel.relationships[key] = {
            "data": filterData[key].data
        };
        return rel;
    };

    /**
     * Filter mock data attributes by set parameters
     * @param {string} type
     * @param {Object} filterData
     * @constructor
     */
    function GetMockDataAttributes(type, filterData) {
        var data = getEntities(type);
        if (filterData) {
            data = _.filter(data, generateRelationshipsData(filterData));
        }

        this.getAttributes = function (attribute) {
            return new getAttributesList(attribute);
        };

        function getAttributesList (attribute) {
            var resData = _.map(data, 'attributes.' + attribute);

            /**
             * Return mock data list in common order
             * @returns {Array}
             */
            this.getInCommonOrder = function () {
                return resData;
            };

            /**
             * Return mock data list in ascending order
             * @returns {Array}
             */
            this.sortAscending = function () {
                return resData.sort();
            };

            /**
             * Return mock data list in descending order
             * @returns {Array}
             */
            this.sortDescending = function () {
                return resData.sort().reverse();
            };
        }
    }

    /**
     * Return mock data list filtered by set parameters
     * @param {string} type
     * @param {Object} filterData
     * @returns {GetMockDataAttributes}
     */
    this.getMockDataAttributes = function (type, filterData) {
        return new GetMockDataAttributes(type, filterData);
    }
};
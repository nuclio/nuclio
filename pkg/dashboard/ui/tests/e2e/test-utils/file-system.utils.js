module.exports = function () {
    var fs = require('fs');
    var testResourcesFolder = e2e_root + "/";

    /**
     * Creates file with given name in test folder
     * @param {string} folder
     * @param {string} name
     */
    this.createFile = function (folder, name) {
        if ( !fs.existsSync(testResourcesFolder + folder) ) {
            fs.mkdirSync(testResourcesFolder + folder);
        }
        fs.writeFile((testResourcesFolder + folder  + "/" + name), new Buffer(1024));
    };

    /**
     * Creates folder with given name
     * @param {string} folder
     */
    this.createFolder = function (folder) {
        if ( !fs.existsSync(testResourcesFolder + folder) ) {
            fs.mkdirSync(testResourcesFolder + folder);
        }
    };

    /**
     * Creates image file encoded in Base64 with given name in test folder
     * @param {string} folder
     * @param {string} name
     * @param {string} image - Base64 encoded image
     */
    this.createImage = function (folder, name, image) {
        if ( !fs.existsSync(testResourcesFolder + folder) ) {
            fs.mkdirSync(testResourcesFolder + folder);
        }
        fs.writeFile((testResourcesFolder + folder  + "/" + name), new Buffer(image, 'base64'));
    };

    this.waitForDownload = function (file_path) {
        browser.driver.wait(function() {
            return fs.existsSync(testResourcesFolder + file_path);
        }, 3000)
    };

    /**
     * Removes file
     * @param {string} file_path
     */
    this.removeFile = function (file_path) {
        if ( fs.existsSync(testResourcesFolder + file_path) ) {
            fs.unlinkSync(testResourcesFolder + file_path);
        }
    };

    /**
     * Removes test-resources folder
     * @param {string} folder
     */
    this.deleteTestResourcesFolder = function (folder) {
        if ( fs.existsSync(testResourcesFolder + folder) ) {
            fs.readdirSync(testResourcesFolder + folder).forEach(function (file,index) {
                var curPath = testResourcesFolder + folder + "/" + file;
                // delete file
                fs.unlinkSync(curPath);
            });
            fs.rmdirSync(testResourcesFolder + folder);
        }
    };
};
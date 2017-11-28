$(function () {
    //
    // Configurations
    //
    var TOAST_DISPLAYED_DURATION = 3000;
    var TOAST_FADE_IN_OUT_DURATION = 200;
    var POLLING_DELAY = 1000;
    var SOURCES_PATH = '/sources';
    var FUNCTIONS_PATH = '/functions';
    var ACE_THEME = 'chrome';
    var CODE_EDITOR_MARGIN = 10;
    var FILTER_BOX_KEY_UP_DEBOUNCE = 100;
    var SPLITTER_ON_DRAG_DEBOUNCE = 350;
    var SPLITTER_GUTTER_SIZE = 3;

    //
    // ACE editor
    //

    /* eslint-disable id-length */
    // A map between file extensions and the ACE mode to use for highlighting files with this extension
    var mapExtToMode = {
        c: 'c_cpp',
        cc: 'c_cpp',
        cpp: 'c_cpp',
        cs: 'csharp',
        css: 'css',
        go: 'golang',
        h: 'c_cpp',
        hpp: 'c_cpp',
        htm: 'html',
        html: 'html',
        java: 'java',
        js: 'javascript',
        json: 'json',
        jsp: 'jsp',
        less: 'less',
        php: 'php',
        pl: 'perl',
        py: 'python',
        rb: 'ruby',
        sh: 'sh',
        sql: 'sql',
        svg: 'svg',
        txt: 'text',
        vb: 'vbscript',
        xml: 'xml',
        yml: 'yaml'
    };
    /* eslint-enable id-length */

    // var model = {
    //     id: '',
    //     metadata: {
    //         annotations: null,
    //         labels: [],
    //         name: '',
    //         namespace: ''
    //     },
    //     spec: {
    //         alias: '',
    //         build: {
    //             addedPaths: [],
    //             baseImageName: '',
    //             commands: [],
    //             functionConfigPath: '',
    //             imageName: '',
    //             imageVersion: '',
    //             noBaseImagesPull: false,
    //             nuclioSourceDir: '',
    //             nuclioSourceURL: '',
    //             outputType: '',
    //             path: '',
    //             registry: '',
    //             scriptPaths: null
    //         },
    //         dataBindings: {},
    //         description: '',
    //         disable: false,
    //         env: null,
    //         handler: '',
    //         httpPort: 0,
    //         image: '',
    //         maxReplicas: 0,
    //         minReplicas: 0,
    //         publish: false,
    //         replicas: 0,
    //         resources: {},
    //         runRegistry: '',
    //         runtime: '',
    //         triggers: {},
    //         version: 0
    //     },
    //     status: {}
    // };

    var codeEditor = createEditor('code-editor', 'text', true, true, false, CODE_EDITOR_MARGIN);
    var inputBodyEditor = createEditor('input-body-editor', 'json', false, false, false, 0);
    var dataBindingsEditor = createEditor('data-bindings-editor', 'json', false, false, false, 0);
    var triggersEditor = createEditor('triggers-editor', 'json', false, false, false, 0);

    /**
     * Creates a new instance of an ACE editor with some enhancements
     * @param {string} id - id of DOM element.
     * @param {string} [mode='text'] - the mode of highlighting. defaults to plain text.
     * @param {boolean} [gutter=false] - `true` for showing gutter (with line numbers).
     * @param {boolean} [activeLine=false] - `true` for highlighting current active line.
     * @param {boolean} [printMargin=false] - `true` for showing print margin.
     * @param {number} [padding=0] - number of pixels to pad the editor text.
     * @returns {Object} new enhanced ACE editor instance
     */
    function createEditor(id, mode, gutter, activeLine, printMargin, padding) {
        var editor = ace.edit(id);
        editor.setTheme('ace/theme/' + ACE_THEME);
        editor.setAutoScrollEditorIntoView(true);
        editor.setHighlightActiveLine(activeLine);
        editor.renderer.setShowGutter(gutter);
        editor.renderer.setShowPrintMargin(printMargin);
        editor.renderer.setScrollMargin(padding, padding, padding, padding);
        setHighlighting(mode);

        return {
            setText: setText,
            getText: getText,
            setHighlighting: setHighlighting,
            disable: disable
        };

        /**
         * Sets the highlighting style of the editor
         * @param {string} [newMode='text'] - the mode of highlighting. defaults to plain text.
         */
        function setHighlighting(newMode) {
            editor.getSession().setMode('ace/mode/' + (newMode || 'text'));
        }

        /**
         * Returns the current contents of the editor
         * @returns {string} the current contents of the editor
         */
        function getText() {
            return editor.getValue();
        }

        /**
         * Sets text in ACE editor
         * @param {string} text - the text to set in the editor
         * @param {string} [newMode='text'] - the mode of highlighting. defaults to plain text
         * @param {boolean} [setFocus=false] - `true` will also set focus on the editor
         */
        function setText(text, newMode, setFocus) {
            editor.setValue(text);
            editor.navigateFileStart();
            if (setFocus) {
                editor.focus();
            }
            setHighlighting(newMode);
        }

        /**
         * Enables or disables editor
         * @param {boolean} [setDisabled=false] - if `true` disables the editor, otherwise enables it
         */
        function disable(setDisabled) {
            editor.setOptions({
                readOnly: setDisabled,
                highlightActiveLine: !setDisabled
            });

            editor.textInput.getElement().disabled = setDisabled;
            editor.renderer.$cursorLayer.element.style.opacity = setDisabled ? 0 : 100;
            editor.container.style.backgroundColor = setDisabled ? '#efeff0' : '#FFFFFF';

            if (setDisabled) {
                editor.setValue('');
            }
        }
    }

    //
    // Utilities
    //

    /**
     * Pretty prints JSON with indentation
     * @param {Object} json - the object to serialized
     * @returns {string} a pretty-print representation of `json` with 4 spaces as indentation
     */
    function printPrettyJson(json) {
        return JSON.stringify(json, null, '    ');
    }

    /**
     * Emits a window resize DOM event
     */
    function emitWindowResize() {
        window.dispatchEvent(new Event('resize'));
    }

    /**
     * Extracts a file name from a provided path
     * @param {string} path - the path including a file name (delimiters: '/' or '\' or both, can be consecutive)
     * @param {boolean} [includeExtension=true] - set to `true` to include extension, or `false` to exclude it
     * @param {boolean} [onlyExtension=false] - set to `true` to include extension only, or `false` to include file name
     * @returns {string} the file name at the end of the given path with or without its extension (depending on the
     *     value of `extension` parameter)
     *
     * @example
     * ```js
     * extractFileName('/path/to/file/file.name.ext');
     * // => 'file.name.ext'
     *
     * extractFileName('\\path/to\\file/file.name.ext', false);
     * // => 'file.name'
     *
     * extractFileName('file.name.ext', false);
     * // => 'file.name'
     *
     * extractFileName('/path/to/////file\\\\\\\\file.name.ext', true);
     * // => 'file.name.ext'
     *
     * extractFileName('/path/to/file\file.name.ext', true, true);
     * // => 'ext'
     *
     * extractFileName('/path/to/file/file.name.ext', false, true);
     * // => ''
     *
     * extractFileName('');
     * // => ''
     *
     * extractFileName(undefined);
     * // => ''
     *
     * extractFileName(null);
     * // => ''
     * ```
     */
    function extractFileName(path, includeExtension, onlyExtension) {
        var start = path.lastIndexOf(_.defaultTo(onlyExtension, false) ? '.' : '/') + 1;
        var end = _.defaultTo(includeExtension, true) ? path.length : path.lastIndexOf('.');
        return _.defaultTo(path, '').replace('\\', '/').substring(start, end);
    }

    //
    // Tabs
    //

    var initialTabIndex = 1;
    var $tabContents = $('#tabs ~ section');
    var $tabHeaders = $('#tabs > ul > li');
    var $selectedTabHeader = $tabHeaders.eq(initialTabIndex);
    var $selectedTabContent = $tabContents.eq(initialTabIndex);

    // register click event handler for tab headers
    $tabHeaders.click(function () {
        // mark old selected tab headers as inactive and hide its corresponding content
        $selectedTabHeader.removeClass('active');
        $selectedTabContent.removeClass('active');

        // change selected tab header to the one the user clicked on
        $selectedTabHeader = $(this);
        $selectedTabContent = $tabContents.eq($tabHeaders.index($selectedTabHeader));

        // mark the new selected tab header as active and show its corresponding content
        $selectedTabHeader.addClass('active');
        $selectedTabContent.addClass('active');
    });

    // on load, first tab is the active one, the rest are hidden
    $tabHeaders.eq(initialTabIndex)[0].click();

    //
    // URL operations
    //

    /**
     * Parses a URL then can get any part of the url: protocol, host, port, path, query-string and hash
     * @param {string} [url=''] - initial URL to parse on creating new parser
     * @returns {Object} the newly created URL parser with `.parse()` and `.get()` methods
     */
    var urlParser = function (url) {
        var anchor = document.createElement('a');
        anchor.href = _.defaultTo(url, '');

        return {

            /**
             * Parses a provided URL
             * @param {string} newUrl - the URL to parse
             */
            parse: function (newUrl) {
                anchor.href = newUrl;
            },

            /**
             * Returns a concatenated string of the parts provided
             * @param {...string} [parts] - the parts to retrieve ('protocol', 'host', 'hostname', 'port', 'pathname').
             *     if a provided part name does not exist then the empty-string will be used in its place.
             * @returns {?string} a string concatenation of all of the parts of the last URL stored with `.parse()`.
             *     if `.parse()` was never called - returns `null`
             */
            get: function () {
                var parts = Array.prototype.slice.call(arguments); // convert `arguments` from Array-like object to Array
                return anchor.href === '' ? null : parts
                    .map(function (part) {
                        return _.get(anchor, part, '');
                    })
                    .join('')
                    .replace(':', '://');
            }
        };
    };

    var loadedUrl = urlParser();
    var workingUrl = getWorkingUrl();

    /**
     * Gets the URL to work with - take the protocol, host and port number from address bar
     * Unless "file" is the protocol, in which case take the default URL: http://52.16.125.41:32050
     * @returns {string} the URL to work with
     */
    function getWorkingUrl() {
        var addressBarUrl = urlParser(window.location.href);
        return addressBarUrl.get('protocol') === 'file://'
            ? 'http://52.16.125.41:32050'
            : addressBarUrl.get('protocol', 'host');
    }

    //
    // Top bar
    //

    var selectedFunction = null;
    var selectedFunctionFileName = '';
    var listRequest = {};
    var $functionList = $('#function-list');
    var $functionListItems = $('#function-list-items');
    var $emptyListMessage = $('#empty-list-message');
    var $loadingMessage = $('#loading-message');
    var $switchFunction = $('#switch-function-button');
    var $functionName = $('#function-name');
    var $functionsFilter = $('#functions-filter');
    var $functionsFilterBox = $('#functions-filter-box');
    var $newName = $('#new-name');
    var $filterClear = $('#filter-clear');
    var $createNew = $('.create-new');
    var $switchFunctionClose = $('#switch-function-close');

    $('.scrollbar-macosx').scrollbar();

    // on page load, hide function list
    closeFunctionList();

    // register click event handler to function switcher - to make it open/close the drop-down menu and toggle its state
    $switchFunction.click(function (event) {
        event.preventDefault();
        event.stopPropagation();

        if ($switchFunction.hasClass('active')) {
            closeFunctionList();
        }
        else {
            $switchFunction.addClass('active');

            $functionList

                // align the left edge of the function drop-down list to the left edge of the switcher
                .css('left', $switchFunction.offset().left)

                // show function drop-down list immediately
                .show(0, function () {
                    // show loading message
                    $loadingMessage.show(0);

                    // register a click event handler for the entire document, to close the function list
                    $(document).click(registerBlurHandler);

                    // fetch function items
                    listRequest = $.ajax(workingUrl + FUNCTIONS_PATH, {
                        method: 'GET',
                        dataType: 'json',
                        contentType: false,
                        processData: false
                    })
                        .done(function (result) {
                            // generate function list
                            generateFunctionMenu(Object.values(result));

                            $loadingMessage.hide(0);

                            // set focus on filter box
                            $functionsFilter.show(0);
                            $functionsFilterBox[0].focus();
                        })
                        .fail(function () {
                            showErrorToast('Failed to retrieve function list...');
                            closeFunctionList();
                        });
                });
        }

        /**
         * Blur event handler for the function list element - when clicking anywhere in the document, outside the
         * function list or its switcher - close the function drop-down list and turn switcher inactive
         * @param {Event} event - the DOM event object of the user click
         */
        function registerBlurHandler(event) {
            if (!_.includes($functionList.find('*').addBack().toArray(), event.target)) {
                closeFunctionList();

                // de-register the click event handler on the entire document until next time the drop-down is open
                $(document).off('click', registerBlurHandler);
            }
        }
    });

    /**
     * Generates the drop-down function menu of the function combo box and display it
     * @param {Array.<Object>} functionList - a list of nuclio functions
     */
    function generateFunctionMenu(functionList) {
        // first, clear the current menu (retain only the "Create new" option)
        $functionListItems.empty();

        // then, for each function from function list (got from response)
        _.forEach(functionList, function (functionItem) {
            // extract file name from path (without its extension)
            var path = _.get(functionItem, 'spec.build.path', '');
            var fileName = extractFileName(path, false); // `false` for "do not include extension"

            // if function item lacks a valid `path` property that ends with a file - skip to next function item
            if (fileName === '') {
                return true;
            }

            // create a new menu item (as a DIV DOM element) ..
            $('<div/>', {

                // .. with the class "option" (for styling only) ..
                'class': 'option',

                // .. with a click event handler that selects the current function and loads it ..
                click: function () {
                    selectedFunction = functionItem; // store selected function
                    setFunctionName(fileName);
                    loadSelectedFunction();
                    closeFunctionList();
                }
            })

                // .. with the file name as the inner text for display ..
                .text(fileName)

                // .. and finally append this menu item to the menu
                .appendTo($functionListItems);

            $functionListItems.show(0);
            return true;
        });

        // if function list is empty - display an appropriate message (otherwise hide it)
        if (functionList.length === 0 || $functionListItems.children().length === 0) {
            $emptyListMessage.show(0);
        }
        else {
            $emptyListMessage.hide(0);
        }
    }

    /**
     * Sets a new function name as the selected function name
     * @param {string} name - name of function to display
     */
    function setFunctionName(name) {
        $functionName
            .text(name)            // display selected function's name in the view
            .removeClass('blank'); // and stop displaying it as blank
    }

    /**
     * Updates the function list by the provided input value in the filter box.
     * If input value is empty, clear button and "Create new" option will be hidden. Otherwise they will be visible.
     */
    function updateFunctionFilter() {
        var inputValue = $functionsFilterBox.val();
        var exactMatch = false;

        // filter function list items by the input value of the filter box (functions whose name starts with that value)
        $functionListItems.children().each(function (index, element) {
            var $element = $(element);
            if (_.startsWith($element.text(), inputValue)) {
                $element.show(0);

                // test if this is an exact match
                if ($element.text() === inputValue) {
                    exactMatch = true;
                }
            }
            else {
                $element.hide(0);
            }
        });

        // if function list is empty after filter, display an appropriate message
        if ($functionListItems.children(':visible').length === 0) {
            $emptyListMessage.show(0);
        }
        else {
            $emptyListMessage.hide(0);
        }

        // if input value of filter box is empty - hide "Create new" option and clear button
        if (inputValue === '') {
            $filterClear.hide(0);
            $createNew.hide(0);
        }

        // otherwise, display clear button
        else {
            $filterClear.show(0);

            // if there was an exact match - hide the "Create new" option
            if (exactMatch) {
                $createNew.hide(0);
            }

            // otherwise, update the "Create new" option's text with the input value of filter box and display it
            else {
                $newName.text(inputValue);
                $createNew.show(0);
            }
        }
    }

    // Register event handler for filter box in function list drop-down, to filter function list on typing in that box
    $functionsFilterBox.keyup(_.debounce(updateFunctionFilter, FILTER_BOX_KEY_UP_DEBOUNCE));

    // Register event handler for clear filter box icon button to clear the filter box input value
    $filterClear.click(function () {
        $functionsFilterBox.val('');
        updateFunctionFilter();
    });

    // Register event handler for click on close button of function list drop-down menu
    $switchFunctionClose.click(closeFunctionList);

    // Register event handler for click on "Create new" option
    $createNew.click(function () {
        var newName = $functionsFilterBox.val();
        closeFunctionList();
        setFunctionName(newName);
    });

    // Register event handler for click on selected function's name - trigger click on "open" button
    $functionName.click(function (event) {
        event.preventDefault();
        event.stopPropagation();
        $switchFunction[0].click();
    });

    /**
     * Closes the function list and turns the function switcher inactice
     */
    function closeFunctionList() {
        // hide function drop-down list
        $newName.text('');
        $functionsFilterBox.val('');
        $emptyListMessage.hide(0);
        $functionsFilter.hide(0);
        $functionListItems.hide(0);
        $createNew.hide(0);
        $functionList.hide(0);
        $filterClear.hide(0);

        // turn function switcher inactive
        $switchFunction.removeClass('active');

        // abort request if it is on-going
        if (_.isFunction(listRequest.abort)) {
            listRequest.abort();
            listRequest = {};
        }
    }

    /**
     * Loads a function's source to the code editor and its settings to the configure/invoke tabs
     */
    function loadSelectedFunction() {
        var path = _.get(selectedFunction, 'spec.build.path', '');
        var fileExtension = extractFileName(path, true, true); // two `true` values for including extension only
        loadSource(path)
            .done(function (responseText) {
                var httpPort             = _.get(selectedFunction, 'spec.httpPort', 0);
                var triggers             = _.get(selectedFunction, 'spec.triggers', {});
                var dataBindings         = _.get(selectedFunction, 'spec.dataBindings', {});
                var environmentVariables = _.get(selectedFunction, 'spec.envs', []);
                var labels               = _.get(selectedFunction, 'metadata.labels', []);

                // omit "name" of each data binding value in selected function's data bindings
                var viewDataBindings = _.mapValues(dataBindings, function (dataBinding) {
                    return _.omit(dataBinding, 'name');
                });

                if (_(viewDataBindings).isEmpty()) {
                    viewDataBindings = {};
                }

                if (typeof responseText === 'string') {
                    loadedUrl.parse(path);
                    terminatePolling();
                    codeEditor.setText(responseText, mapExtToMode[fileExtension], true);
                    disableInvokeTab(httpPort === 0);
                    dataBindingsEditor.setText(printPrettyJson(viewDataBindings), 'json');
                    triggersEditor.setText(printPrettyJson(triggers), 'json');
                    labels.setKeyValuePairs(labels);
                    envVars.setKeyValuePairs(environmentVariables);
                    showSuccessToast('Source loaded successfully!');
                }
                else {
                    showErrorToast('Source is not textual...');
                }
            })
            .fail(function () {
                showErrorToast('Source failed to load...');
            });
    }

    // Register event handler for "Save" button in top bar
    var $saveButton = $('#save-function');
    $saveButton.click(function () {
        var url = workingUrl + SOURCES_PATH + '/' + selectedFunctionFileName;
        saveSource(url)
            .done(function () {
                loadedUrl.parse(url);
                deployFunction();
            })
            .fail(function () {
                showErrorToast('Deploy failed...');
            });
    });

    //
    // Function operations (load/save/deploy/invoke)
    //

    /**
     * Loads a source file
     * @param {string} url - the url of the source to load
     * @returns {Promise} a promise resolving with the source at `url` on successful response, or rejecting on response
     *     failure
     */
    function loadSource(url) {
        return $.ajax(url, {
            method: 'GET',
            dataType: 'text',
            contentType: false,
            processData: false
        });
    }

    /**
     * Saves a source file from the editor
     * @param {string} url - the url of the source to load
     * @returns {Promise} a promise resolving on successful response, or rejecting on response failure
     */
    function saveSource(url) {
        var content = codeEditor.getText();
        return $.ajax(url, {
            method: 'POST',
            data: content,
            contentType: false,
            processData: false
        });
    }

    /**
     * Builds a function from a source file
     */
    function deployFunction() {
        var url = loadedUrl.get('pathname');

        if (url !== null) {
            var dataBindings = dataBindingsEditor.getText();
            var triggers = triggersEditor.getText();
            var path = loadedUrl.get('pathname');
            var name = extractFileName(path, false); // `false` for "do not include extension"

            try {
                dataBindings = JSON.parse(dataBindings);
            }
            catch (error) {
                showErrorToast('Failed to parse data bindings...');
                return;
            }

            try {
                triggers = JSON.parse(triggers);
            }
            catch (error) {
                showErrorToast('Failed to parse triggers...');
                return;
            }

            // disable Invoke tab, until function is successfully deployed
            disableInvokeTab(true);

            // initiate deploy process
            $.ajax(loadedUrl.get('protocol', 'host') + FUNCTIONS_PATH, {
                method: 'POST',
                dataType: 'json',
                data: JSON.stringify({
                    metadata: {
                        name: name,
                        labels: labels.getKeyValuePairs(),
                        namespace: $('#namespace').val()
                    },
                    spec: {
                        build: {
                            baseImageName: $('#base-image').val(),
                            commands: _.without($('#commands').val().replace('\r', '\n').split('\n'), ''),
                            path: url,
                            registry: ''
                        },
                        dataBindings: _.defaultTo(dataBindings, null),
                        description: $('#description').val(),
                        disable: !$('#enabled').val(),
                        env: envVars.getKeyValuePairs(),
                        triggers: _.defaultTo(triggers, null)
                    }
                }),
                contentType: false,
                processData: false
            })
                .done(function () {
                    showSuccessToast('Deploy started successfully!');
                    startPolling(name);
                })
                .fail(function () {
                    showErrorToast('Deploy failed...');
                });
        }
    }

    /**
     * Invokes a function with some input and displays its output
     */
    function invokeFunction() {
        var path = '/' + _.trimStart($('#input-path').val(), '/ ');
        var httpPort = _.get(selectedFunction, 'spec.httpPort', 0);
        var url = workingUrl + '/tunnel/' + loadedUrl.get('hostname') + ':' + httpPort + path;
        var method = $('#input-method').val();
        var contentType = isFileInput ? false : $inputContentType.val();
        var body = isFileInput ? new FormData($invokeFile.get(0)) : inputBodyEditor.getText();
        var level = $('#input-level').val();
        var logs = [];
        var output = '';

        $.ajax(url, {
            method: method,
            data: body,
            dataType: 'text',
            cache: false,
            contentType: contentType,
            processData: false,
            beforeSend: function (xhr) {
                xhr.setRequestHeader('x-nuclio-log-level', level);
            }
        })
            .done(function (data, textStatus, jqXHR) {
                // parse logs from "x-nuclio-logs" response header
                var logsString = extractResponseHeader(jqXHR.getAllResponseHeaders(), 'x-nuclio-logs', '[]');

                try {
                    logs = JSON.parse(logsString);
                }
                catch (error) {
                    console.error('Error parsing "x-nuclio-logs" response header as a JSON:\n' + error.message);
                    logs = [];
                }

                // attempt to parse response body as JSON, if fails - parse as text
                try {
                    output = printPrettyJson(JSON.parse(data));
                }
                catch (error) {
                    output = data;
                }

                printToLog(jqXHR);
            })
            .fail(function (jqXHR) {
                output = jqXHR.responseText;
                printToLog(jqXHR);
            });

        /**
         * Appends the status code, headers and body of the response to current logs, and prints them to log
         * @param {Object} jqXHR - the jQuery XHR object
         */
        function printToLog(jqXHR) {
            var emptyMessage = '&lt;empty&gt;';
            logs.push({
                time: Date.now(),
                level: 'info',
                message: 'Function invocation response: ' +
                '<pre>' +
                '\n\n&gt; Status code:\n' + jqXHR.status + ' ' + jqXHR.statusText +
                '\n\n&gt; Headers:\n' + (_(jqXHR.getAllResponseHeaders()).trimEnd('\n') || emptyMessage) +
                '\n\n&gt; Body:\n' +
                (output || emptyMessage) + '\n\n' +
                '</pre>'
            });

            appendToLog(logs);
        }

        /**
         * Extracts a header from a newline separated list of headers
         * @param {string} allResponseHeaders - a newline separated list of key-value pairs of headers (name: value)
         * @param {string} headerToExtract - the name of the header to extract
         * @param {string} [notFoundValue=''] - the value to return in case the header was not found in the list
         * @returns {string} the value of the header to extract if it is found, or default value otherwise
         */
        function extractResponseHeader(allResponseHeaders, headerToExtract, notFoundValue) {
            var headers = allResponseHeaders.split('\n');
            var foundHeader = _(headers).find(function (header) {
                return _(header.toLowerCase()).startsWith(headerToExtract.toLowerCase() + ':');
            });

            return _(foundHeader).isUndefined()
                ? _(notFoundValue).defaultTo('')
                : foundHeader.substr(foundHeader.indexOf(':') + 1).trim();
        }
    }

    //
    // "Configure" tab
    //

    // init key-value pair inputs
    dataBindingsEditor.setText('{}'); // initially data-bindings should be an empty object
    triggersEditor.setText('{}'); // initially triggers should be an empty object
    var labels = createKeyValuePairsInput('labels');
    var envVars = createKeyValuePairsInput('env-vars');

    /**
     * Creates a new key-value pairs input
     * @param {string} id - the "id" attribute of some DOM element in which to populate this component
     * @param {Object} [initial={}] - the initial key-value pair list
     * @returns {{getKeyValuePairs: getKeyValuePairs, setKeyValuePairs: setKeyValuePairs}} the component has two methods
     *     for getting and setting the inner key-value pairs object
     */
    function createKeyValuePairsInput(id, initial) {
        var pairs = _(initial).defaultTo({});

        var $container = $('#' + id);
        var headers =
            '<li class="headers">' +
            '<span class="pair-key">Key</span>' +
            '<span class="pair-value">Value</span>' +
            '</li>';

        $container.html(
            '<ul id="' + id + '-pair-list" class="pair-list"></ul>' +
            '<div id="' + id + '-add-new-pair-form" class="add-new-pair-form">' +
            '<input type="text" class="text-input new-key" id="' + id + '-new-key" placeholder="Type key...">' +
            '<input type="text" class="text-input new-value" id="' + id + '-new-value" placeholder="Type value...">' +
            '<button class="add-pair-button" title="Add" id="' + id + '-add-new-pair">+</button>' +
            '</div>'
        );

        var $pairList = $('#' + id + '-pair-list');
        var $newKeyInput = $('#' + id + '-new-key');
        var $newValueInput = $('#' + id + '-new-value');
        var $newPairButton = $('#' + id + '-add-new-pair');
        $newPairButton.click(addNewPair);

        redraw(); // draw for the first time

        return {
            getKeyValuePairs: getKeyValuePairs,
            setKeyValuePairs: setKeyValuePairs
        };

        // public methods

        /**
         * Retrieves the current key-value pair list
         * @returns {Object} the current key-pair list as an object
         */
        function getKeyValuePairs() {
            return pairs;
        }

        /**
         * Sets the current key-value pair list to the provided one
         * @param {Object} [newObject={}] - key-pair list will be set to this object
         */
        function setKeyValuePairs(newObject) {
            pairs = _.defaultTo(newObject, {});
            redraw();
        }

        // private methods

        /**
         * Adds a new key-value pair according to user input
         */
        function addNewPair() {
            var key = $newKeyInput.val();
            var value = $newValueInput.val();

            // if either "Key" or "Value" input fields are empty - set focus on the empty one
            if (_(key).isEmpty()) {
                $newKeyInput[0].focus();
                showErrorToast('Key is empty...');
            }
            else if (_(value).isEmpty()) {
                $newValueInput[0].focus();
                showErrorToast('Value is empty...');

                // if key already exists - set focus and select the contents of "Key" input field and display message
            }
            else if (_(pairs).has(key)) {
                $newKeyInput[0].focus();
                $newKeyInput[0].select();
                showErrorToast('Key already exists...');

                // otherwise - all is valid
            }
            else {
                // set the new value at the new key
                pairs[key] = value;

                // redraw list in the view with new added key-value pair
                redraw();

                // clear "Key" and "Value" input fields and set focus to "Key" input field - for next input
                $newKeyInput.val('');
                $newValueInput.val('');
                $newKeyInput[0].focus();
            }
        }

        /**
         * Removes the key-value pair identified by `key`
         * @param {string} key - the key by which to identify the key-value pair to be removed
         */
        function removePairByKey(key) {
            delete pairs[key];
            redraw();
        }

        /**
         * Redraws the key-value list in the view
         */
        function redraw() {
            // unbind event handlers from DOM elements before removing them
            $pairList.find('[class=remove-pair-button]').each(function () {
                $(this).off('click');
            });

            // remove all DOM of pair list
            $pairList.empty();

            // if there are currently no pairs on the list - display an appropriate message
            if (_(pairs).isEmpty()) {
                $pairList.append('<li>Empty list. You may add new entries.</li>');
            }

            // otherwise - build HTML for list of key-value pairs, plus add headers
            else {
                $pairList.append('<li>' + _(pairs).map(function (value, key) {
                    return '<span class="pair-key text-ellipsis" title="' + key + '">' + key + '</span>' +
                           '<span class="pair-value text-ellipsis" title="' + value + '">' + value + '</span>';
                }).join('</li><li>') + '</li>');

                var listItems = $pairList.find('li'); // all list items

                // for each key-value pair - append a remove button to its list item DOM element
                listItems.each(function () {
                    var $listItem = $(this);
                    var key = $listItem.find('[class^=pair-key]').text();
                    $('<button/>', {
                        'class': 'remove-pair-button',
                        title: 'Remove',
                        click: function () {
                            removePairByKey(key);
                        }
                    })
                        .html('&times;')
                        .appendTo($listItem);
                });

                // prepend the headers list item before the data list items
                $pairList.prepend(headers);
            }
        }
    }

    //
    // "Invoke" tab
    //

    var $invokeTabElements = $('#invoke-section').find('select, input, button');
    var $invokeInputBody = $('#input-body-editor');
    var $invokeFile = $('#input-file');
    var isFileInput = false;

    // initially hide file input field
    $invokeFile.hide(0);

    // Register event handler for "Send" button in "Invoke" tab
    $('#input-send').click(invokeFunction);

    // Register event handler for "Clear log" hyperlink
    $('#clear-log').click(clearLog);

    // Register event handler for "Method" drop-down list in "Invoke" tab
    // if method is GET then editor is disabled
    var $inputMethod = $('#input-method');
    $inputMethod.change(function () {
        var disable = $inputMethod.val() === 'GET';
        inputBodyEditor.disable(disable);
    });

    // Register event handler for "Content type" drop-down list in "Invoke" tab
    var $inputContentType = $('#input-content-type');
    var mapContentTypeToMode = {
        'text/plain': 'text',
        'application/json': 'json'
    };
    $inputContentType.change(function () {
        var mode = mapContentTypeToMode[$inputContentType.val()];
        isFileInput = _.isUndefined(mode);
        if (isFileInput) {
            $invokeInputBody.hide(0);
            $invokeFile.show(0);
        }
        else {
            inputBodyEditor.setHighlighting(mode);
            $invokeInputBody.show(0);
            $invokeFile.hide(0);
        }
    });

    /**
     * Enables or disables all controls in "Invoke" tab
     * @param {boolean} [disable=false] - if `true` then controls will be disabled, otherwise they will be enabled
     */
    function disableInvokeTab(disable) {
        $invokeTabElements.prop('disabled', disable);
        inputBodyEditor.disable(disable);
        hideFunctionUrl(disable);
    }

    /**
     * Hides/shows function execution's URL
     * @param {boolean} [hide=false] - `true` for hiding, otherwise showing
     */
    function hideFunctionUrl(hide) {
        var httpPort = _.get(selectedFunction, 'spec.httpPort', 0);
        $('#input-url').html(hide ? '' : loadedUrl.get('protocol', 'hostname') + ':' + httpPort);
    }

    // initially disable all controls
    disableInvokeTab(true);

    //
    // Log
    //

    var $log = $('#log'); // log DOM element
    var $logSection = $('#log-section'); // log section DOM element
    var lastTimestamp = -Infinity; // remembers the latest timestamp of last chunk of log entries

    /**
     * Appends lines of log entries to log
     * @param {Array.<Object>} logEntries - a list of log entries
     * @param {string} logEntries[].message - the essence of the log entry, describing what happened
     * @param {string} logEntries[].level - either one of 'debug', 'info', 'warn' or 'error', indicating severity
     * @param {number} logEntries[].time - timestamp of log entry in milliseconds since epoch (1970-01-01T00:00:00Z)
     * @param {string} [logEntries[].err] - on failure, describes the error
     */
    function appendToLog(logEntries) {
        var newEntries = _.filter(logEntries, function (logEntry) {
            return logEntry.time > lastTimestamp;
        });

        if (!_(newEntries).isEmpty()) {
            lastTimestamp = _(newEntries).maxBy('time').time;
            _.forEach(newEntries, function (logEntry) {
                var timestamp = new Date(Math.floor(logEntry.time)).toISOString();
                var levelDisplay = '[' + logEntry.level.toUpperCase() + ']';
                var errorMessage = _.get(logEntry, 'err', '');
                var customParameters = _.omit(logEntry, ['name', 'time', 'level', 'message', 'err']);
                var html = '<div>' + timestamp + '&nbsp;<span class="level-' + logEntry.level + '">' +
                    levelDisplay + '</span>:&nbsp;' + logEntry.message +
                    (_(errorMessage).isEmpty() ? '' : '&nbsp;<span>' + errorMessage + '</span>') +
                    (_(customParameters).isEmpty() ? '' : ' [' + _.map(customParameters, function (value, key) {
                        return key + ': ' + JSON.stringify(value);
                    }).join(', ') + ']') +
                    '</div>';
                $log.append(html);
                $logSection.scrollTop($logSection.prop('scrollHeight')); // scroll to bottom of log
            });
        }
    }

    /**
     * Clears the log
     */
    function clearLog() {
        $log.html('');
    }

    //
    // Polling
    //

    var pollingDelayTimeout = null; // timeout for polling (delay between instances of polling)

    /**
     * Terminates polling
     */
    function terminatePolling() {
        if (pollingDelayTimeout !== null) {
            window.clearTimeout(pollingDelayTimeout);
            pollingDelayTimeout = null;
        }

        lastTimestamp = -Infinity;
    }

    /**
     * Initiates polling on a function to get its state
     * @param {string} name - the name of the function to poll
     */
    function startPolling(name) {
        // poll once immediately
        poll();

        /**
         * A single polling iteration
         * Gets the function status and logs it
         */
        function poll() {
            var url = loadedUrl.get('protocol', 'host') + FUNCTIONS_PATH + '/' + name;
            $.ajax(url, {
                method: 'GET',
                dataType: 'json'
            })
                .done(function (pollResult) {
                    appendToLog(_.get(pollResult, 'status.logs', []));

                    if (shouldKeepPolling(pollResult)) {
                        pollingDelayTimeout = window.setTimeout(poll, POLLING_DELAY);
                    }
                    else if (pollResult.state === 'Ready') {
                        if (selectedFunction === null) {
                            selectedFunction = {};
                        }

                        // store the port for newly created function
                        var httpPort = _.get(pollResult, 'spec.httpPort', 0);
                        _.set(selectedFunction, 'spec.httpPort', httpPort);

                        // enable controls of "Invoke" tab and display a message about it
                        disableInvokeTab(false);
                        showSuccessToast('You can now invoke the function!');
                    }
                });
        }
    }

    /**
     * Tests whether or not polling should be continuing
     * @param {Object} pollResult - the result retrieved from polling
     * @returns {boolean} `true` if polling should continue, or `false` otherwise
     */
    function shouldKeepPolling(pollResult) {
        var firstWord = pollResult.state.split(/\s+/)[0];
        return !['Ready', 'Failed'].includes(firstWord);
    }

    //
    // Toast methods
    //

    var toastTimeout = null; // common timeout for toast messages
    var $toast = $('#toast'); // toast DOM element

    $toast.hide(0);

    /**
     * Clears the timeout for hiding toast
     */
    function clearToastTimeout() {
        if (toastTimeout !== null) {
            window.clearTimeout(toastTimeout);
            toastTimeout = null;
        }
    }

    /**
     * Shows an error toast message
     * @param {string} message - the message to display
     */
    function showErrorToast(message) {
        showToast(message, 'error', TOAST_DISPLAYED_DURATION);
    }

    /**
     * Shows a success toast message
     * @param {string} message - the message to display
     */
    function showSuccessToast(message) {
        showToast(message, 'success', TOAST_DISPLAYED_DURATION);
    }

    /**
     * Shows a toast message (overrides current displayed toast message if there is one)
     * @param {string} message - the message to display
     * @param {string} clazz - the CSS class to set for the toast (it will replace all existing classes)
     * @param {number} [duration] - if provided, toast will be hidden after this amount of milli-seconds
     */
    function showToast(message, clazz, duration) {
        clearToastTimeout();
        $toast.removeClass()
            .addClass(clazz)
            .text(message)
            .fadeIn(TOAST_FADE_IN_OUT_DURATION);

        if ($.isNumeric(duration)) {
            toastTimeout = window.setTimeout(hideToast, duration);
        }
    }

    /**
     * Hides the toast message
     */
    function hideToast() {
        clearToastTimeout();
        $toast.fadeOut(TOAST_FADE_IN_OUT_DURATION, function () {
            $toast.text('');
        });
    }

    //
    // Splitters
    //

    /* eslint-disable no-magic-numbers */
    /* eslint-disable new-cap */
    Split(['#upper', '#footer'], {
        sizes: [60, 40],
        minSize: [250, 100],
        gutterSize: SPLITTER_GUTTER_SIZE,
        snapOffset: 0,
        direction: 'vertical',
        onDrag: _.debounce(emitWindowResize, SPLITTER_ON_DRAG_DEBOUNCE)
    });

    Split(['#editor-section', '#invoke-section'], {
        sizes: [60, 40],
        minSize: [200, 500],
        gutterSize: SPLITTER_GUTTER_SIZE,
        snapOffset: 0,
        onDrag: _.debounce(emitWindowResize, SPLITTER_ON_DRAG_DEBOUNCE)
    });
    /* eslint-enable no-magic-numbers */
    /* eslint-enable new-cap */
});

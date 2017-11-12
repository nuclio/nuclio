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
    var COMBO_BOX_KEY_UP_DEBOUNCE = 1000;
    var SPLITTER_ON_DRAG_DEBOUNCE = 350;
    var SPLITTER_GUTTER_SIZE = 5;

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

    var codeEditor = createEditor('editor', 'text', true, true, false, CODE_EDITOR_MARGIN);
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

    //
    // Tabs
    //

    var tabContents = $('#tabs ~ section');
    var tabHeaders = $('#tabs > ul > li');
    var selectedTabHeader = tabHeaders.first();

    // register click event handler for tab headers
    tabHeaders.click(function () {
        // mark old selected tab headers as inactive and hide its corresponding content
        selectedTabHeader.removeClass('active');
        tabContents.eq(tabHeaders.index(selectedTabHeader)).css('top', '-9999px');

        // change selected tab header to the one the user clicked on
        selectedTabHeader = $(this);

        // mark the new selected tab header as active and show its corresponding content
        selectedTabHeader.addClass('active');
        tabContents.eq(tabHeaders.index(selectedTabHeader)).css('top', '36px');
    });

    // on load, first tab is the active one, the rest are hidden
    tabContents.css('top', '-9999px');
    tabHeaders.first()[0].click();

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
    var functionListElement = $('#function-list');
    var selectFunctionElement = $('#select-function');

    // Register event handler for focusing on function combo box (load function list and open drop-down)
    selectFunctionElement.focus(function () {
        var url = workingUrl + FUNCTIONS_PATH;
        return $.ajax(url, {
            method: 'GET',
            dataType: 'json',
            contentType: false,
            processData: false
        })
            .done(function (result) {
                generateFunctionMenu(Object.values(result));
            })
            .fail(function () {
                showErrorToast('Failed to retrieve function list...');
            });
    });

    // register click event handler to arrow of combo box - to make it open the drop down too
    $('#arrow').click(function () {
        selectFunctionElement[0].focus();
    });

    // register event handler for changing "URL" input field - to update the highlighting of the code editor
    selectFunctionElement.keyup(_.debounce(function () {
        var fileExtension = selectFunctionElement.val().split('.').pop();
        codeEditor.setHighlighting(mapExtToMode[fileExtension]);
    }, COMBO_BOX_KEY_UP_DEBOUNCE)); // will be triggered only after some time since last typing anything in the box

    // on page load, hide function list, then focus on combo box to make the list open for the first time
    functionListElement.hide(0);
    selectFunctionElement.focus();

    /**
     * Generates the drop-down function menu of the function combo box and display it
     * @param {Array.<Object>} functionList - a list of nuclio functions
     */
    function generateFunctionMenu(functionList) {
        // first, clear the current menu
        functionListElement.html('');

        // then, for each function from function list (got from response)
        functionList.forEach(function (functionItem) {
            // get source URL
            var sourceUrl = functionItem.source_url;

            // extract file name from URL (the substring to the right of the last '/' character in URL)
            var fileName = sourceUrl.split('/').pop();

            // create a new menu item (as a DIV DOM element) ..
            $('<div/>', {

                // .. with the class "option" (for styling only) ..
                'class': 'option',

                // .. with a click event handler that selects the current function and loads it ..
                click: function () {
                    selectedFunction = functionItem;
                    selectFunctionElement.val(fileName); // display the selected name
                    loadSelectedFunction();
                }
            })

                // .. with file name as the inner text for display ..
                .text(fileName)

                // .. and finally append this menu item to the menu
                .appendTo(functionListElement);
        });

        // if function list is empty - display a message
        if (functionList.length === 0) {
            functionListElement.append('<div class="not-found">No function found. You can deploy a new one!</div>');
        }

        functionListElement

            // set the minimum width of the function drop-down list to the width of the text input
            .css('min-width', selectFunctionElement.outerWidth())

            // show function drop-down list immediately
            .show(0, function () {
                // then register a click event handler for the entire document
                $(document).click(registerBlurHandler);
            });

        /**
         * Blur event handler for the function list element - when clicking anywhere in the document, outside the
         * "select function" combo box input - close the function drop-down list
         * @param {Event} event - the DOM event object of the user click
         */
        function registerBlurHandler(event) {
            if (event.target !== functionListElement[0] && event.target !== selectFunctionElement[0]) {
                // hide function drop-down list
                functionListElement.hide(0);

                // de-register the click event handler on the entire document until next time the drop-down is open
                $(document).off('click', registerBlurHandler);
            }
        }
    }

    /**
     * Loads a function's source to the code editor and its settings to the configure/invoke tabs
     */
    function loadSelectedFunction() {
        var fileExtension = selectedFunction.source_url.split('/').pop().split('.').pop();
        loadSource(selectedFunction.source_url)
            .done(function (responseText) {
                var triggers = _.defaultTo(selectedFunction.triggers, {});

                // omit "name" of each data binding value in selected function's data bindings
                var dataBindings = _.mapValues(selectedFunction.data_bindings, function (dataBinding) {
                    return _.omit(dataBinding, 'name');
                });

                if (_(dataBindings).isEmpty()) {
                    dataBindings = {};
                }

                if (typeof responseText === 'string') {
                    loadedUrl.parse(selectedFunction.source_url);
                    terminatePolling();
                    codeEditor.setText(responseText, mapExtToMode[fileExtension], true);
                    disableInvokeTab(selectedFunction.node_port === 0);
                    dataBindingsEditor.setText(printPrettyJson(dataBindings), 'json');
                    triggersEditor.setText(printPrettyJson(triggers), 'json');
                    labels.setKeyValuePairs(selectedFunction.labels);
                    envVars.setKeyValuePairs(selectedFunction.envs);
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

    // Register event handler for "Deploy" button in top bar
    $('#deploy').click(function () {
        var url = workingUrl + SOURCES_PATH + '/' + selectFunctionElement.val();
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
            var name = path.substr(path.lastIndexOf('/') + 1); // last part of URL after last forward-slash character
            if (_(name).includes('.')) {
                name = name.split('.')[0]; // get rid of file extension
            }

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
                    name: name,
                    source_url: url,
                    registry: '',
                    data_bindings: _.defaultTo(dataBindings, {}),
                    triggers: _.defaultTo(triggers, {}),
                    labels: labels.getKeyValuePairs(),
                    envs: envVars.getKeyValuePairs()
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
        var url = workingUrl + '/tunnel/' + loadedUrl.get('hostname') + ':' + selectedFunction.node_port + path;
        var method = $('#input-method').val();
        var contentType = isFileInput ? false : inputContentType.val();
        var body = isFileInput ? new FormData(invokeFileElement.get(0)) : inputBodyEditor.getText();
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

        var container = $('#' + id);
        var headers =
            '<li class="headers">' +
            '<span class="pair-key">Key</span>' +
            '<span class="pair-value">Value</span>' +
            '</li>';

        container.html(
            '<ul id="' + id + '-pair-list" class="pair-list"></ul>' +
            '<div id="' + id + '-add-new-pair-form" class="add-new-pair-form">' +
            '<input type="text" class="text-input new-key" id="' + id + '-new-key" placeholder="Type key...">' +
            '<input type="text" class="text-input new-value" id="' + id + '-new-value" placeholder="Type value...">' +
            '<button class="add-pair-button" title="Add" id="' + id + '-add-new-pair">+</button>' +
            '</div>'
        );

        var pairList = $('#' + id + '-pair-list');
        var newKeyInput = $('#' + id + '-new-key');
        var newValueInput = $('#' + id + '-new-value');
        var newPairButton = $('#' + id + '-add-new-pair');
        newPairButton.click(addNewPair);

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
            var key = newKeyInput.val();
            var value = newValueInput.val();

            // if either "Key" or "Value" input fields are empty - set focus on the empty one
            if (_(key).isEmpty()) {
                newKeyInput[0].focus();
                showErrorToast('Key is empty...');
            }
            else if (_(value).isEmpty()) {
                newValueInput[0].focus();
                showErrorToast('Value is empty...');

                // if key already exists - set focus and select the contents of "Key" input field and display message
            }
            else if (_(pairs).has(key)) {
                newKeyInput[0].focus();
                newKeyInput[0].select();
                showErrorToast('Key already exists...');

                // otherwise - all is valid
            }
            else {
                // set the new value at the new key
                pairs[key] = value;

                // redraw list in the view with new added key-value pair
                redraw();

                // clear "Key" and "Value" input fields and set focus to "Key" input field - for next input
                newKeyInput.val('');
                newValueInput.val('');
                newKeyInput[0].focus();
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
            pairList.find('[class=remove-pair-button]').each(function () {
                $(this).off('click');
            });

            // remove all DOM of pair list
            pairList.html('');

            // if there are currently no pairs on the list - display an appropriate message
            if (_(pairs).isEmpty()) {
                pairList.append('<li>Empty list. You may add new entries.</li>');

                // otherwise - build HTML for list of key-value pairs, plus add headers
            }
            else {
                pairList.append('<li>' + _(pairs).map(function (value, key) {
                    return '<span class="pair-key text-ellipsis" title="' + key + '">' + key + '</span>' +
                           '<span class="pair-value text-ellipsis" title="' + value + '">' + value + '</span>';
                }).join('</li><li>') + '</li>');

                var listItems = pairList.find('li'); // all list items

                // for each key-value pair - append a remove button to its list item DOM element
                listItems.each(function () {
                    var listItem = $(this);
                    var key = listItem.find('[class^=pair-key]').text();
                    $('<button/>', {
                        'class': 'remove-pair-button',
                        title: 'Remove',
                        click: function () {
                            removePairByKey(key);
                        }
                    })
                        .text('x')
                        .appendTo(listItem);
                });

                // prepend the headers list item before the data list items
                pairList.prepend(headers);
            }
        }
    }

    //
    // "Invoke" tab
    //

    var invokeTabElements = $('#invoke-section').find('select, input, button');
    var invokeInputBodyElement = $('#input-body-editor');
    var invokeFileElement = $('#input-file');
    var isFileInput = false;

    // initially hide file input field
    invokeFileElement.hide(0);

    // Register event handler for "Send" button in "Invoke" tab
    $('#input-send').click(invokeFunction);

    // Register event handler for "Clear log" hyperlink
    $('#clear-log').click(clearLog);

    // Register event handler for "Method" drop-down list in "Invoke" tab
    // if method is GET then editor is disabled
    var inputMethodElement = $('#input-method');
    inputMethodElement.change(function () {
        var disable = inputMethodElement.val() === 'GET';
        inputBodyEditor.disable(disable);
    });

    // Register event handler for "Content type" drop-down list in "Invoke" tab
    var inputContentType = $('#input-content-type');
    var mapContentTypeToMode = {
        'text/plain': 'text',
        'application/json': 'json'
    };
    inputContentType.change(function () {
        var mode = mapContentTypeToMode[inputContentType.val()];
        isFileInput = _.isUndefined(mode);
        if (isFileInput) {
            invokeInputBodyElement.hide(0);
            invokeFileElement.show(0);
        }
        else {
            inputBodyEditor.setHighlighting(mode);
            invokeInputBodyElement.show(0);
            invokeFileElement.hide(0);
        }
    });

    /**
     * Enables or disables all controls in "Invoke" tab
     * @param {boolean} [disable=false] - if `true` then controls will be disabled, otherwise they will be enabled
     */
    function disableInvokeTab(disable) {
        invokeTabElements.prop('disabled', disable);
        inputBodyEditor.disable(disable);
        hideFunctionUrl(disable);
    }

    /**
     * Hides/shows function execution's URL
     * @param {boolean} [hide=false] - `true` for hiding, otherwise showing
     */
    function hideFunctionUrl(hide) {
        $('#input-url').html(hide ? '' : loadedUrl.get('protocol', 'hostname') + ':' + selectedFunction.node_port);
    }

    // initially disable all controls
    disableInvokeTab(true);

    //
    // Log
    //

    var logElement = $('#log'); // log DOM element
    var logSectionElement = $('#log-section'); // log section DOM element
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
                        return key + ': ' + value;
                    }).join(', ') + ']') +
                    '</div>';
                logElement.append(html);
                logSectionElement.scrollTop(logSectionElement.prop('scrollHeight')); // scroll to bottom of log
            });
        }
    }

    /**
     * Clears the log
     */
    function clearLog() {
        logElement.html('');
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
                    appendToLog(pollResult.logs);

                    if (shouldKeepPolling(pollResult)) {
                        pollingDelayTimeout = window.setTimeout(poll, POLLING_DELAY);
                    }
                    else if (pollResult.state === 'Ready') {
                        if (selectedFunction === null) {
                            selectedFunction = {};
                        }

                        // store the port for newly created function
                        selectedFunction.node_port = pollResult.node_port;

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
    var toastElement = $('#toast'); // toast DOM element

    toastElement.hide(0);

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
        toastElement.removeClass()
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
        toastElement.fadeOut(TOAST_FADE_IN_OUT_DURATION, function () {
            toastElement.text('');
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

    Split(['#editor-section', '#right-pane'], {
        sizes: [60, 40],
        minSize: [200, 500],
        gutterSize: SPLITTER_GUTTER_SIZE,
        snapOffset: 0,
        onDrag: _.debounce(emitWindowResize, SPLITTER_ON_DRAG_DEBOUNCE)
    });
    /* eslint-enable no-magic-numbers */
    /* eslint-enable new-cap */
});

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
    var SPLITTER_GUTTER_SIZE = 5;
    var SPLITTER_SNAP_OFFSET = 100;

    var KEY_CODES = {
        TAB: 9,
        ENTER: 13,
        ESC: 27,
        UP: 38,
        DOWN: 40
    };

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
    //     metadata: {
    //         labels: {},
    //         name: '',
    //         namespace: ''
    //     },
    //     spec: {
    //         alias: '',
    //         build: {
    //             baseImageName: '',
    //             commands: [],
    //             path: '',
    //             registry: '',
    //         },
    //         dataBindings: {},
    //         description: '',
    //         disable: false,
    //         env: [],
    //         httpPort: 0,
    //         maxReplicas: 0,
    //         minReplicas: 0,
    //         replicas: 0,
    //         triggers: {},
    //     }
    // };

    var codeEditor = createEditor('code-editor', 'text', true, true, false, CODE_EDITOR_MARGIN);
    var inputBodyEditor = createEditor('input-body-editor', 'json', false, false, false, 0);

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

    /**
     * Registers a click event handler on the entire document, so when the user clicks anywhere in teh document except
     * for the given root element (and its descendants) then the callback function is invoked, and the event handler
     * is de-registered.
     * @param {jQuery} rootElement - the element for which clicking will *not* invoke the `callback`
     * @param {function} callback - the function to invoke whenever the user clicks anywhere except for `rootElement`
     * @param {number} [keyCode] - a keyboard key-code to trigger blur in addition to click event
     */
    function createBlurHandler(rootElement, callback, keyCode) {
        $(document).click(registerClickBlurHandler);

        if (_.isNumber(keyCode)) {
            $(document).keyup(registerKeyBlurHandler);
        }

        /**
         * Registers click event handler for the the entire document, so when clicking anywhere in the document,
         * outside the provided root element - the callback function will be called
         * @param {Event} event - the DOM event object of the user click
         */
        function registerClickBlurHandler(event) {
            if (!_.includes(rootElement.find('*').addBack().toArray(), event.target)) {
                callback();

                // de-register the click event handler on the entire document until next time the drop-down is open
                $(document).off('click', registerClickBlurHandler);
            }
        }

        /**
         * Registers key-up event handler for the the entire document, so when pressing and releasing a key whose
         * key-code is the provided one - the callback function will be called
         * @param {Event} event - the DOM event object of the user key-up
         */
        function registerKeyBlurHandler(event) {
            if (event.which === keyCode) {
                callback();

                // de-register the click event handler on the entire document until next time the drop-down is open
                $(document).off('keyup', registerKeyBlurHandler);
            }
        }
    }

    /**
     * Clears the input fields of a provided element. Text/number/text-area inputs will become empty, drop-down menus
     * will get their first option selected, and checkboxes will get un-checked.
     * @param {jQuery} $element - the element whose input field to clear
     */
    function clearInputs($element) {
        $element.find('input:not([type=checkbox]),textarea').addBack('input:not([type=checkbox]),textarea').val('');
        $element.find('input[type=checkbox]').addBack('input[type=checkbox]').prop('checked', false);
        $element.find('select option:eq(0)').addBack('select option:eq(0)').prop('selected', true);
    }

    //
    // Tabs
    //

    var initialTabIndex = 0;
    var $tabContents = $('#main > section');
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
    $tabHeaders.eq(initialTabIndex).get(0).click();

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

    // Maps between runtime and the corresponding file extension and display label
    var runtimeConf = {
        'python:2.7': {
            extension: 'py',
            label: 'Python 2.7'
        },
        'python:3.6': {
            extension: 'py',
            label: 'Python 3.6'
        },
        'pypy': {
            extension: 'py',
            label: 'PyPy'
        },
        'golang': {
            extension: 'go',
            label: 'Go'
        },
        'shell': {
            extension: 'sh',
            label: 'Shell'
        },
        'nodejs': {
            extension: 'js',
            label: 'NodeJS'
        }
    };
    var selectedFunction = null;
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
    var $createNewRuntime = $('#switch-function-create-new-runtime');
    var $createNewButton = $('#switch-function-create-new-button');
    var $switchFunctionClose = $('#switch-function-close');
    var $deployButton = $('#deploy-function');

    // initialize nice scroll-bar for function drop-down menu
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
                    createBlurHandler($functionList, closeFunctionList, KEY_CODES.ESC);

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
                            $functionsFilterBox.get(0).focus();
                        })
                        .fail(function () {
                            showErrorToast('Failed to retrieve function list...');
                            closeFunctionList();
                        });
                });
        }
    });

    /**
     * Generates the drop-down function menu of the function combo box and display it
     * @param {Array.<Object>} functionList - a list of nuclio functions
     */
    function generateFunctionMenu(functionList) {
        // first, clear the current menu (retain only the "Create new" option)
        $functionListItems.empty();

        // regarding function list (got from response) ..
        functionList

            // .. filter out function items that lack a name or path attributes (as they are mandatory)
            .filter(function (functionItem) {
                return _.has(functionItem, 'metadata.name') && _.has(functionItem, 'spec.build.path');
            })

            // .. then, for each function item
            .forEach(function (functionItem) {
                var name = _.get(functionItem, 'metadata.name');
                var runtime = _.get(functionItem, 'spec.runtime', '');
                var runtimeLabel = _.get(runtimeConf, [runtime, 'label'], '');

                // create a new menu item (as a DIV DOM element) ..
                $('<div/>', {

                    // .. with the class "option" (for styling only) ..
                    'class': 'option',

                    // .. with a click event handler that selects the current function and loads it ..
                    'click': function () {
                        selectedFunction = functionItem; // store selected function
                        setFunctionName(name);
                        loadSelectedFunction();
                        closeFunctionList();
                    },

                    'mouseover': function () {
                        $functionListItems.children().removeClass('focus');
                    }
                })

                    // .. with the file name as the inner text for display ..
                    .text(name + (runtimeLabel === '' ? '' : ' (' + runtimeLabel + ')'))

                    // .. and finally append this menu item to the menu
                    .appendTo($functionListItems);

                $functionListItems.show(0);
            });

        // if function list is empty - display an appropriate message (otherwise hide it)
        if (functionList.length === 0 || $functionListItems.children().length === 0) {
            $emptyListMessage.show(0);
        }
        else {
            $emptyListMessage.hide(0);
        }

        $(document).keydown(_.debounce, navigateFunctionList, FILTER_BOX_KEY_UP_DEBOUNCE);
    }

    /**
     * Navigates through the list options with up/down arrow keys, and select the focused one with Enter key.
     * @param {Event} event - the key down event
     */
    function navigateFunctionList(event) {
        // get currently selected option among all list items
        var $options = $functionListItems.children();
        var currentFocusedOption = findFocusedOption();
        var currentFocusedIndex = $options.index(currentFocusedOption);
        var nextFocusedIndex = -1;

        // make all options not focused
        $options.removeClass('focus');

        // if up/down arrow keys are pressed - determine the next option to focus on
        if (event.which === KEY_CODES.DOWN) {
            nextFocusedIndex = (currentFocusedIndex + 1) % $options.length;
        }
        else if (event.which === KEY_CODES.UP) {
            nextFocusedIndex = (currentFocusedIndex - 1) % $options.length;
        }

        // if the Enter key is pressed - call the click event handler for this option
        else if (event.which === KEY_CODES.ENTER) {
            currentFocusedOption.get(0).click();
        }

        // if any other key is pressed - do nothing
        else {
            return;
        }

        // get the option that needs to be focused - and set it as focused and scroll it into view if it is not visible
        var $optionToFocus = $options.eq(nextFocusedIndex);
        $optionToFocus.addClass('focus');
        if ($optionToFocus.offset().top >
            $functionsFilterBox.offset().top + $functionListItems.scrollTop() + $functionListItems.height() ||
            $optionToFocus.offset().top + $optionToFocus.height() < $functionListItems.offset().top) {
            $optionToFocus.get(0).scrollIntoView();
        }

        /**
         * Find current focused/hovered option:
         * 1. if an option is focused using keyboard navigation - it is returned; otherwise
         * 2. if an option is hovered by the mouse cursor - it is returned; otherwise
         * 3. an empty `jQuery` set is returned
         * @returns {jQuery} the relevant focused option by the above logic
         *
         * @private
         */
        function findFocusedOption() {
            var $result = $functionListItems.find('.focus');
            return $result.length === 0 ? $functionListItems.find(':hover') : $result;
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

        $deployButton.prop('disabled', false);
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

    // Register event handler for click on "Create" button in function lost drop-down menu
    $createNewButton.click(function () {
        var name = $functionsFilterBox.val();
        var runtime = $createNewRuntime.val();
        createNewFunction(name, runtime);
    });

    // Register event handler for click on selected function's name - trigger click on "open" button
    $functionName.click(function (event) {
        event.preventDefault();
        event.stopPropagation();
        $switchFunction.get(0).click();
    });

    /**
     * Creates a new blank function with the provided name
     * @param {string} name - the function name
     * @param {string} runtime - the runtime environment for the function (also determines the extension of the file)
     */
    function createNewFunction(name, runtime) {
        var extension = runtimeConf[runtime].extension;
        closeFunctionList();
        setFunctionName(name);
        selectedFunction = {
            metadata: { name: name },
            spec: {
                build: { path: SOURCES_PATH + '/' + name + '.' + extension },
                runtime: runtime
            }
        };
        clearAll();
        codeEditor.setHighlighting(mapExtToMode[extension]);
    }

    /**
     * Clears the entire web-app - all input fields are cleared (view and model)
     */
    function clearAll() {
        // "Code" tab
        disableInvokePane(true);
        loadedUrl.parse('');
        clearLog();
        codeEditor.setText('');
        codeEditor.setHighlighting();

        // "Configure" tab
        clearInputs($('#configure-tab'));
        configDataBindings.clear();
        configEnvVars.clear();
        configLabels.clear();
        configRuntimeAttributes.clear();

        // "Triggers" tab
        triggersInput.clear();
    }

    /**
     * Closes the function list and turns the function switcher inactive
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

        // unbind "keydown" event handler for navigating through the function list items
        $(document).off('keydown', navigateFunctionList);
    }

    /**
     * Loads a function's source to the code editor and its settings to the "Configure"/"Triggers" tabs
     */
    function loadSelectedFunction() {
        var path = _.get(selectedFunction, 'spec.build.path', '');
        var fileExtension = extractFileName(path, true, true); // two `true` values for including extension only
        loadSource(path)
            .done(function (responseText) {
                var enabled              = !_.get(selectedFunction, 'spec.disable', false);
                var httpPort             = _.get(selectedFunction, 'spec.httpPort', 0);
                var triggers             = _.get(selectedFunction, 'spec.triggers', {});
                var dataBindings         = _.get(selectedFunction, 'spec.dataBindings', {});
                var runtimeAttributes    = _.get(selectedFunction, 'spec.runtimeAttributes', {});
                var environmentVariables = _.get(selectedFunction, 'spec.env', {});
                var commands             = _.get(selectedFunction, 'spec.build.commands', []);
                var baseImage            = _.get(selectedFunction, 'spec.build.baseImageName', '');
                var description          = _.get(selectedFunction, 'spec.description', '');
                var handler              = _.get(selectedFunction, 'spec.handler', '');
                var labels               = _.get(selectedFunction, 'metadata.labels', {});
                var namespace            = _.get(selectedFunction, 'metadata.namespace', '');

                if (typeof responseText === 'string') {
                    loadedUrl.parse(path);
                    terminatePolling();
                    codeEditor.setText(responseText, mapExtToMode[fileExtension], true);
                    disableInvokePane(httpPort === 0);
                    $('#handler').val(handler);
                    $('#commands').text(commands.join('\n'));
                    $('#base-image').val(baseImage);
                    $('#enabled').prop('checked', enabled);
                    $('#description').val(description);
                    $('#namespace').val(namespace);
                    configLabels.setKeyValuePairs(labels);
                    configEnvVars.setKeyValuePairs(_.mapValues(_.keyBy(environmentVariables, 'name'), 'value'));
                    configRuntimeAttributes.setKeyValuePairs(runtimeAttributes);
                    configDataBindings.setKeyValuePairs(dataBindings);
                    triggersInput.setKeyValuePairs(triggers);
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
    $deployButton.click(function () {
        var path = _.get(selectedFunction, 'spec.build.path');
        var url = workingUrl + path;

        if (_.isEmpty(path)) {
            showErrorToast('Deploy failed...');
        }
        else {
            saveSource(url)
                .done(function () {
                    loadedUrl.parse(url);
                    deployFunction();
                })
                .fail(function () {
                    showErrorToast('Deploy failed...');
                });
        }
    });

    //
    // Create new function pop-up
    //

    var $createNewPopUp = $('#create-new-pop-up');
    var $createNewName = $('#create-new-name');

    // Register "New" button click event handler for opening the "New function" pop-up and set focus on the name input
    $('#create-new-button').click(function (event) {
        event.stopPropagation();
        $createNewPopUp.show(0);
        $createNewName.get(0).focus();
        createBlurHandler($createNewPopUp, $createNewPopUp.hide.bind($createNewPopUp, 0), KEY_CODES.ESC);
    });

    // Register "Create" button click event handler for applying the pop-up and creating a new function
    $('#create-new-apply').click(function () {
        var name = $createNewName.val();
        var runtime = $('#create-new-runtime').val();

        if (_(name).isEmpty()) {
            showErrorToast('Name is empty...');
        }
        else {
            createNewFunction(name, runtime);
            $createNewPopUp.hide(0);
        }
    });

    // Register click event handler for close button to close pop-up
    $('#create-new-close').click(function () {
        $createNewPopUp.hide(0);
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
        var path = _.get(selectedFunction, 'spec.build.path', '');
        var name = _.get(selectedFunction, 'metadata.name', '');

        // path and name are mandatory for a function - make sure they exist before continuing
        if (path !== '' && name !== '') {
            // convert view values to model values
            _.merge(selectedFunction, {
                metadata: {
                    labels: configLabels.getKeyValuePairs(),
                    namespace: $('#namespace').val()
                },
                spec: {
                    build: {
                        baseImageName: $('#base-image').val(),
                        commands: _.without($('#commands').val().replace('\r', '\n').split('\n'), ''),
                        path: path,
                        registry: ''
                    },
                    dataBindings: configDataBindings.getKeyValuePairs(),
                    runtimeAttributes: configRuntimeAttributes.getKeyValuePairs(),
                    description: $('#description').val(),
                    disable: !$('#enabled').val(),
                    env: _.map(configEnvVars.getKeyValuePairs(), function (value, key) {
                        return {
                            name: key,
                            value: value
                        };
                    }),
                    handler: generateHandler(),
                    triggers: triggersInput.getKeyValuePairs()
                }
            });

            // populate conditional properties
            populatePort();

            // disable "Invoke" pane, until function is successfully deployed
            disableInvokePane(true);

            // initiate deploy process
            $.ajax(workingUrl + FUNCTIONS_PATH, {
                method: 'POST',
                dataType: 'json',
                data: JSON.stringify(selectedFunction),
                contentType: 'application/json',
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

        /**
         * Populate `spec.httpPort` if a trigger of kind `'http'` exists and have a `port` attribute
         *
         * @private
         */
        function populatePort() {
            var httpPort = _.chain(triggersInput.getKeyValuePairs())
                .pickBy(['kind', 'http'])
                .values()
                .first()
                .get('attributes.port')
                .value();

            // if HTTP trigger was added, inject its port number to the functions `httpPort` property
            if (_.isNumber(httpPort)) {
                _.set(selectedFunction, 'spec.httpPort', httpPort);
            }
        }

        /**
         * Generates value for `spec.handler` property by the following logic:
         * If "Handler" text box is empty or includes a colon ":" then use it as-is.
         * If "Handler" text box is non-empty but does not include a colon ":" then prepend it with function's name
         * followed by a colon ":".
         *
         * If runtime is shell, the handler is returned as-is.
         *
         * @returns {string} the handler value to use for deploying function
         *
         * @private
         */
        function generateHandler() {
            var handler = $('#handler').val();

            // for now, shell has some funky handler rules. work around this by not doing any fancy
            // augmentation if the runtime is shell
            if (selectedFunction.spec.runtime === 'shell') {
                return handler;
            }

            return (handler === '' || handler.includes(':')) ? handler : name + ':' + handler;
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
        var body = isFileInput ? $invokeFile.get(0).files.item(0) : inputBodyEditor.getText();
        var contentType = isFileInput ? body.type : $inputContentType.val();
        var dataType = isFileInput ? 'binary' : 'text';
        var level = $('#input-level').val();
        var logs = [];
        var output = '';

        $.ajax(url, {
            method: method,
            data: body,
            dataType: dataType,
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

                if (isFileInput) {
                    var urlCreator = window.URL || window.webkitURL;
                    var blobUrl = urlCreator.createObjectURL(data);
                    output = '<a class="download-link" href="' + blobUrl + '" download="' + body.name + '" ' +
                        'title="Download the binary response as a file">Download</a>\n';
                    if (_(contentType).startsWith('image/')) {
                        output += '<img src="' + blobUrl + '" alt="Image response" title="Image response">\n';
                    }
                }
                else {
                    // attempt to parse response body as JSON, if fails - parse as text
                    try {
                        output = printPrettyJson(JSON.parse(data));
                    }
                    catch (error) {
                        output = data;
                    }
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
    // "Code " tab
    //

    // Drag'n'Drop textual files into the code editor
    var validFileExtensions = ['.py', '.pypy', '.go', '.sh', '.txt'];

    var $codeEditor = $('#code-editor');
    var $codeEditorDropZone = $('#code-editor-drop-zone');
    $codeEditor
        .on('dragover', null, false)
        .on('dragenter', null, function (event) {
            $codeEditorDropZone.addClass('dragover');
            $codeEditor.css('opacity', '0.4');
            event.preventDefault();
        })
        .on('dragleave', null, function (event) {
            $codeEditorDropZone.removeClass('dragover');
            $codeEditor.css('opacity', '');
            event.preventDefault();
        })
        .on('drop', null, function (event) {
            var itemType = _.get(event, 'originalEvent.dataTransfer.items[0].type');
            var file = _.get(event, 'originalEvent.dataTransfer.files[0]');
            var extension = extractFileName(file.name, true, true);

            if (isFileDropValid(itemType, extension)) {
                var reader = new FileReader();
                reader.onload = function (event) {
                    codeEditor.setText(event.target.result);
                    $codeEditorDropZone.removeClass('dragover');
                    $codeEditor.css('opacity', '');
                };
                reader.onerror = function () {
                    showErrorToast('Could not read file...');
                };
                reader.readAsText(file);
            }
            else {
                $codeEditorDropZone.removeClass('dragover');
                $codeEditor.css('opacity', '');
                showErrorToast('Invalid file type/extension');
            }
            event.preventDefault();
        });

    /**
     * Tests whether a file is valid for dropping in code editor according to its MIME type and its extension
     * @param {string} type - the MIME type of the file (e.g. 'text/plain', 'application/javascript')
     * @param {string} extension - the extension of the file (e.g. 'txt', 'py', 'html')
     * @returns {boolean} `true` if the file is valid for dropping in code editor, or `false` otherwise
     */
    function isFileDropValid(type, extension) {
        return _(type).startsWith('text/') || validFileExtensions.includes(extension);
    }

    //
    // "Configure" tab
    //

    // init key-value pair inputs
    var configLabels = createKeyValuePairsInput('labels');
    var configEnvVars = createKeyValuePairsInput('env-vars');
    var configRuntimeAttributes = createKeyValuePairsInput('runtime-attributes', undefined, undefined, undefined, {
        getValue: function (id) {
            var val = $('#' + id + '-new-value').val();

            if (!_.isNaN(Number(val))) {
                return Number(val);
            }

            if (_(val).startsWith('{') || _(val).startsWith('[')) {
                try {
                    return JSON.parse(val);
                }
                catch (error) {
                    return val;
                }
            }

            return val;
        },
        parseValue: function (value) {
            if (_.isNumber(value)) {
                return value;
            }

            if (_.isObject(value)) {
                try {
                    return '<pre>' + printPrettyJson(value).replace(/"/g, '&quot;') + '</pre>';
                }
                catch (error) {
                    return value;
                }
            }

            return value;
        }
    });
    var configDataBindings = createKeyValuePairsInput('config-data-bindings', {}, 'name', 'attributes', {
        getTemplate: function () {
            return '<ul id="config-data-bindings-new-value"><li><select id="config-data-bindings-class" class="dropdown">' +
                       '<option value="v3io">v3io</option>' +
                   '</select></li>' +
                   '<li><input class="text-input" id="config-data-bindings-url" placeholder="URL..."></li>' +
                   '<li><input class="text-input" id="config-data-bindings-secret" placeholder="Secret..."></li></ul>';
        },
        getValue: function () {
            return {
                'class':  $('#config-data-bindings-class').val(),
                'url':    $('#config-data-bindings-url').val(),
                'secret': $('#config-data-bindings-secret').val()
            };
        },
        isValueEmpty: function () {
            return _($('#config-data-bindings-class').val()).isEmpty() ||
                _($('#config-data-bindings-secret').val()).isEmpty();
        },
        parseValue: function (value) {
            return 'Class: ' + value['class'] + ' | URL: ' + value.url + ' | Secret: ' + value.secret;
        },
        setFocusOnValue: function () {
            if (_($('#config-data-bindings-url').val()).isEmpty()) {
                $('#config-data-bindings-url').get(0).focus();
            }
            else {
                $('#config-data-bindings-secret').get(0).focus();
            }
        },
        clearValue: function () {
            $('#config-data-bindings-class option').eq(0).prop('selected', true);
            $('#config-data-bindings-url').val('');
            $('#config-data-bindings-secret').val('');
        }
    });

    /**
     * Creates a new key-value pairs input
     * @param {string} id - the "id" attribute of some DOM element in which to populate this component
     * @param {Object} [initial={}] - the initial key-value pair list
     * @param {Object} [keyHeader='key'] - the text for column header to use for key, as well as the placeholder
     * @param {Object} [valueHeader='value'] - the text for column header to use for value, as well as the placeholder
     * @param {Object} [valueManipulator] - allows generic use of key-value pairs for more complex values
     *     if omitted, the default value manipulator will take place (regular single string value)
     * @param {function} [valueManipulator.getTemplate] - returns an HTML template as a string, or a jQuery object to
     *     append to the DOM as the input for a new value
     * @param {function} [valueManipulator.getValue] - converts view (DOM input fields from `.getTemplate()`) to model
     * @param {function} [valueManipulator.isValueEmpty] - tests whether the input is considered empty or not
     * @param {function} [valueManipulator.parseValue] - converts model to view for display on key-value list
     * @param {function} [valueManipulator.setFocusOnValue] - sets focus on relevant input field
     * @param {function} [valueManipulator.clearValue] - resets the input fields
     * @returns {{getKeyValuePairs: function, setKeyValuePairs: function, clear: function}} the component has two
     *      methods for getting and setting the inner key-value pairs object, and a method to clear the entire component
     */
    function createKeyValuePairsInput(id, initial, keyHeader, valueHeader, valueManipulator) {
        var pairs = _(initial).defaultTo({});
        var headers = {
            key: _(keyHeader).defaultTo('key'),
            value: _(valueHeader).defaultTo('value')
        };
        var vManipulator = getValueManipulator();

        var $container = $('#' + id);
        var headersHtml =
            '<li class="headers space-between">' +
            '<span class="pair-key">' + headers.key + '</span>' +
            '<span class="pair-value">' + headers.value + '</span>' +
            '<span class="pair-action">&nbsp;</span>' +
            '</li>';

        var template = vManipulator.getTemplate();
        $container.html(
            '<ul id="' + id + '-pair-list" class="pair-list"></ul>' +
            '<div id="' + id + '-add-new-pair-form" class="add-new-pair-form space-between">' +
            '<div class="new-key"><input type="text" class="text-input new-key" id="' + id + '-new-key" placeholder="Type ' + headers.key + '..."></div>' +
            '<div class="new-value">' + (_.isString(template) ? template : '') + '</div>' +
            '<button class="pair-action add-pair-button button green" title="Add" id="' + id + '-add-new-pair">+</button>' +
            '</div>'
        );

        var $pairList = $('#' + id + '-pair-list');
        var $newKeyInput = $('#' + id + '-new-key');
        var $newValueInput = $container.find('.new-value');
        var $newPairButton = $('#' + id + '-add-new-pair');
        $newPairButton.click(addNewPair);

        if (template instanceof jQuery) {
            template.appendTo($newValueInput);
        }

        redraw(); // draw for the first time

        return {
            getKeyValuePairs: getKeyValuePairs,
            setKeyValuePairs: setKeyValuePairs,
            clear: clear
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

        /**
         * Clears the entire component: both the key-value list (view & model) and the form input fields
         */
        function clear() {
            clearInput();
            setKeyValuePairs({});
        }

        // private methods

        /**
         * Returns a value manipulator. Each of its properties defaults to the default manipulator, and can be
         * overridden if the corresponding property has a function value defined in external provided value-manipulator.
         * Properties in external value-manipulator that are not one of the documented ones are ignored.
         * @returns {{getTemplate: function, getValue: function, isValueEmpty: function, parseValue: function,
         * setFocusOnValue: function, clearValue: function}} value-manipulator with default properties except for
         * possible overridden properties by the external provided value-manipulator
         *
         * @private
         */
        function getValueManipulator() {
            var defaultManipulator = {
                getTemplate: _.constant('<input type="text" class="text-input" id="' + id +
                    '-new-value" placeholder="Type ' + headers.value + '...">'),
                getValue: function () {
                    return $('#' + id + '-new-value').val();
                },
                isValueEmpty: function () {
                    return _.isEmpty($('#' + id + '-new-value').val());
                },
                parseValue: _.identity,
                setFocusOnValue: function () {
                    $('#' + id + '-new-value').get(0).focus();
                },
                clearValue: function () {
                    $('#' + id + '-new-value').val('');
                }
            };

            return _.chain(valueManipulator)
                .pick(['getTemplate', 'getValue', 'isValueEmpty', 'parseValue', 'setFocusOnValue', 'clearValue'])
                .pickBy(_.isFunction)
                .defaults(defaultManipulator)
                .value();
        }

        /**
         * Clears "Key" and "Value" input fields and set focus to "Key" input field - for next input
         */
        function clearInput() {
            vManipulator.clearValue();
            $newKeyInput.val('').get(0).focus();
        }

        /**
         * Adds a new key-value pair according to user input
         *
         * @private
         */
        function addNewPair() {
            var key = $newKeyInput.val();

            // if either "Key" or "Value" input fields are empty - set focus on the empty one
            if (_(key).isEmpty()) {
                $newKeyInput.get(0).focus();
                showErrorToast(headers.key + ' is empty...');
            }
            else if (vManipulator.isValueEmpty()) {
                vManipulator.setFocusOnValue();
                showErrorToast(headers.value + ' is empty...');
            }

            // if key already exists - set focus and select the contents of "Key" input field and display message
            else if (_(pairs).has(key)) {
                $newKeyInput.get(0).focus();
                $newKeyInput.get(0).select();
                showErrorToast(headers.key + ' already exists...');
            }

            // otherwise - all is valid
            else {
                // set the new value at the new key
                pairs[key] = vManipulator.getValue(id);

                // redraw list in the view with new added key-value pair
                redraw();

                // clear input and make ready for input of next key-value pair
                clearInput();
            }
        }

        /**
         * Removes the key-value pair identified by `key`
         * @param {string} key - the key by which to identify the key-value pair to be removed
         *
         * @private
         */
        function removePairByKey(key) {
            delete pairs[key];
            redraw();
        }

        /**
         * Redraws the key-value list in the view
         *
         * @private
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
                $pairList.append('<li class="space-between">' + _(pairs).map(function (value, key) {
                    return '<span class="pair-key text-ellipsis" title="' + key + '">' + key + '</span>' +
                           '<span class="pair-value text-ellipsis" title="' + vManipulator.parseValue(value) + '">' +
                            vManipulator.parseValue(value) + '</span>';
                }).join('</li><li class="space-between">') + '</li>');

                var $listItems = $pairList.find('li'); // all list items

                // for each key-value pair - append a remove button to its list item DOM element
                $listItems.each(function () {
                    var $listItem = $(this);
                    var $key = $listItem.find('.pair-key');
                    var $value = $listItem.find('.pair-value');

                    $('<button/>', {
                        'class': 'pair-action remove-pair-button button red',
                        'title': 'Remove',
                        'click': function () {
                            removePairByKey($key.text());
                        }
                    })
                        .html('&times;')
                        .appendTo($listItem);

                    $key.click(function () {
                        $(this).toggleClass('text-ellipsis');
                        $(this).toggleClass('wrap-around');
                    });

                    $value.click(function () {
                        $(this).toggleClass('text-ellipsis');
                        $(this).toggleClass('wrap-around');
                    });
                });

                // prepend the headers list item before the data list items
                $pairList.prepend(headersHtml);
            }
        }
    }

    //
    // "Invoke" pane
    //

    var $invokePaneElements = $('#invoke-section').find('select, input, button');
    var $invokeInputBody = $('#input-body-editor');
    var $invokeFile = $('#input-file');
    var isFileInput = false;

    // initially hide file input field
    $invokeFile.hide(0);

    // Register event handler for "Send" button in "Invoke" pane
    $('#input-send').click(invokeFunction);

    // Register event handler for "Clear log" hyperlink
    $('#clear-log').click(clearLog);

    // Register event handler for "Method" drop-down list in "Invoke" pane
    // if method is GET then editor is disabled
    var $inputMethod = $('#input-method');
    $inputMethod.change(function () {
        var disable = $inputMethod.val() === 'GET';
        inputBodyEditor.disable(disable);
    });

    // Register event handler for "Content type" drop-down list in "Invoke" pane
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
     * Enables or disables all controls in "Invoke" pane
     * @param {boolean} [disable=false] - if `true` then controls will be disabled, otherwise they will be enabled
     */
    function disableInvokePane(disable) {
        $invokePaneElements.prop('disabled', disable);
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
    disableInvokePane(true);

    //
    // "Log" pane
    //

    var $log = $('#log'); // log DOM element
    var $logSection = $('#log-section'); // log section DOM element

    /**
     * Appends lines of log entries to log
     * @param {Array.<Object>} logEntries - a list of log entries
     * @param {string} logEntries[].message - the essence of the log entry, describing what happened
     * @param {string} logEntries[].level - either one of 'debug', 'info', 'warn' or 'error', indicating severity
     * @param {number} logEntries[].time - timestamp of log entry in milliseconds since epoch (1970-01-01T00:00:00Z)
     * @param {string} [logEntries[].err] - on failure, describes the error
     */
    function appendToLog(logEntries) {
        if (!_(logEntries).isEmpty()) {
            _.forEach(logEntries, function (logEntry) {
                var timestamp = new Date(Math.floor(logEntry.time)).toISOString();
                var levelDisplay = '[' + logEntry.level.toUpperCase() + ']';
                var errorMessage = _.get(logEntry, 'err', '');
                var customParameters = _.omit(logEntry, ['name', 'time', 'level', 'message', 'err']);
                var html = '<div>' + timestamp + '&nbsp;<span class="level-' + logEntry.level + '">' +
                    levelDisplay + '</span>:&nbsp;' + logEntry.message +
                    (_(errorMessage).isEmpty() ? '' : '&nbsp;<span>' + errorMessage + '</span>') +
                    (_(customParameters).isEmpty() ? '' : ' [' + _.map(customParameters, function (value, key) {
                        return key + ': ' + JSON.stringify(value);
                    }).join(', ').replace(/\\n/g, '\n').replace(/\\"/g, '"') + ']') +
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
        $log.empty();
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
    }

    /**
     * Initiates polling on a function to get its state
     * @param {string} name - the name of the function to poll
     */
    function startPolling(name) {
        var lastTimestamp = -Infinity;

        // poll once immediately
        poll();

        /**
         * A single polling iteration
         * Gets the function status and logs it
         */
        function poll() {
            var url = workingUrl + FUNCTIONS_PATH + '/' + name;
            $.ajax(url, {
                method: 'GET',
                dataType: 'json'
            })
                .done(function (pollResult) {
                    var logs = _.get(pollResult, 'status.logs', []).filter(function (logEntry) {
                        return lastTimestamp < logEntry.time;
                    });

                    if (!_(logs).isEmpty()) {
                        lastTimestamp = _(logs).maxBy('time').time;
                        appendToLog(logs);
                    }

                    if (shouldKeepPolling(pollResult)) {
                        pollingDelayTimeout = window.setTimeout(poll, POLLING_DELAY);
                    }
                    else if (_.get(pollResult, 'status.state') === 'Ready') {
                        if (selectedFunction === null) {
                            selectedFunction = {};
                        }

                        // store the port for newly created function
                        var httpPort = _.get(pollResult, 'spec.httpPort', 0);
                        _.set(selectedFunction, 'spec.httpPort', httpPort);

                        // enable controls of "Invoke" pane and display a message about it
                        disableInvokePane(false);
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
        var firstWord = _.get(pollResult, 'status.state', '').split(/\s+/)[0];
        return !['Ready', 'Failed'].includes(firstWord);
    }

    //
    // "Triggers" tab
    //

    /**
     * Creates a value manipulator for "Triggers" value part, in order to use it with the key-value input component
     * @returns {{getTemplate: function, getValue: function, isValueEmpty: function, parseValue: function,
     *     setFocusOnValue: function, clearValue: function}} returns the required methods for value manipulator
     */
    function createTriggersValueManipulator() {
        var $component = null;
        var $kind = null;
        var $triggersFields = null;

        // a description of the different properties where:
        // `path`  - the path to the property in the value object
        // `id`    - the "id" attribute of the DOM element bound to this property
        // `type`  - the type of the property in the value object (Array means an array of numbers from ranges, '1-2,3')
        // `label` - the label for this property when displaying it in the view
        var properties = [
            {
                id: 'triggers-enabled',
                path: 'disabled',
                type: Boolean,
                label: 'Enabled',
                kinds: ['http', 'rabbit-mq', 'kafka', 'kinesis', 'nats']
            },
            {
                id: 'triggers-partitions',
                path: 'attributes.partitions',
                type: toNumberArray,
                label: 'Partitions',
                kinds: ['kafka', 'v3ioItemPoller']
            },
            {
                id: 'triggers-http-workers',
                path: 'maxWorkers',
                type: Number,
                label: 'Max workers',
                kinds: ['http']
            },
            {
                id: 'triggers-v3io-paths',
                path: 'paths',
                type: toStringArray,
                label: 'Paths',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-url',
                path: 'url',
                type: String,
                label: 'URL',
                kinds: ['kafka', 'nats', 'v3ioItemPoller']
            },
            {
                id: 'triggers-total',
                path: 'numPartitions',
                type: Number,
                label: 'Total',
                kinds: ['kafka', 'kinesis', 'v3ioItemPoller']
            },
            {
                id: 'triggers-topic',
                path: 'attributes.topic',
                type: String,
                label: 'Topic',
                kinds: ['kafka', 'nats']
            },
            {
                id: 'triggers-http-host',
                path: 'attributes.ingresses.http.host',
                type: String,
                label: 'Host',
                kinds: ['http']
            },
            {
                id: 'triggers-http-paths',
                path: 'attributes.ingresses.http.paths',
                type: toStringArray,
                label: 'Paths',
                kinds: ['http']
            },
            {
                id: 'triggers-http-port',
                path: 'attributes.port',
                type: Number,
                label: 'External port',
                kinds: ['http']
            },
            {
                id: 'triggers-rabbitmq-exchange',
                path: 'attributes.exchangeName',
                type: String,
                label: 'Exchange name',
                kinds: ['rabbit-mq']
            },
            {
                id: 'triggers-rabbitmq-queue',
                path: 'attributes.queueName',
                type: String,
                label: 'Queue name',
                kinds: ['rabbit-mq']
            },
            {
                id: 'triggers-kinesis-key',
                path: 'attributes.accessKeyID',
                type: String,
                label: 'Access key ID',
                kinds: ['kinesis']
            },
            {
                id: 'triggers-kinesis-secret',
                path: 'attributes.secretAccessKey',
                type: String,
                label: 'Secret access key',
                kinds: ['kinesis']
            },
            {
                id: 'triggers-kinesis-region',
                path: 'attributes.regionName',
                type: String,
                label: 'Region',
                kinds: ['kinesis']
            },
            {
                id: 'triggers-kinesis-stream',
                path: 'attributes.streamName',
                type: String,
                label: 'Stream',
                kinds: ['kinesis']
            },
            {
                id: 'triggers-kinesis-shards',
                path: 'attributes.shards',
                type: toNumberArray,
                label: 'Shards',
                kinds: ['kinesis']
            },
            {
                id: 'triggers-v3io-interval',
                path: 'attributes.intervalMs',
                type: Number,
                label: 'Interval (ms)',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-batch-size',
                path: 'attributes.maxBatchSize',
                type: Number,
                label: 'Max Batch Size',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-batch-wait',
                path: 'attributes.maxBatchWaitMs',
                type: Number,
                label: 'Max Batch Wait (ms)',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-restart',
                path: 'attributes.restart',
                type: Boolean,
                label: 'Restart',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-incremental',
                path: 'attributes.incremental',
                type: Boolean,
                label: 'Incremental',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-attributes',
                path: 'attributes.attributes',
                type: toStringArray,
                label: 'Attributes',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-queries',
                path: 'attributes.queries',
                type: toStringArray,
                label: 'Queries',
                kinds: ['v3ioItemPoller']
            },
            {
                id: 'triggers-v3io-suffixes',
                path: 'attributes.suffixes',
                type: toStringArray,
                label: 'Suffixes',
                kinds: ['v3ioItemPoller']
            }
        ];

        return {
            getTemplate: function () {
                $component = $('<ul id="triggers-new-value">' +
                    '<li><label><input type="checkbox" id="triggers-enabled" title="Enable/disable trigger"> Enabled</label></li>' +
                    '<li><select id="triggers-kind" class="dropdown" title="Each trigger kind has a different set of fields to fill">' +
                        '<option value="">Select kind...</option>' +
                        '<option value="http">HTTP</option>' +
                        '<option value="rabbit-mq">RabbitMQ</option>' +
                        '<option value="kafka">Kafka</option>' +
                        '<option value="kinesis">Kinesis</option>' +
                        '<option value="nats">NATS</option>' +
                        '<option value="v3ioItemPoller">v3io</option>' +
                    '</select></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-v3io-paths" class="text-input" title="Paths (e.g. path1, path2)" placeholder="Paths, e.g. path1, path2..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-url" class="text-input" title="URL" placeholder="URL..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-topic" class="text-input" title="Topic" placeholder="Topic..."></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-total" class="text-input" min="0" title="Total number of partitions/shards" placeholder="Total shards/partitions..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-partitions" class="text-input" title="Partitions (e.g. 1,2-3,4)" placeholder="Partitions, e.g. 1,2-3,4" pattern="\\s*\\d+(\\s*-\\s*\\d+)?(\\s*,\\s*\\d+(\\s*-\\s*\\d+)?)*(\\s*(,\\s*)?)?"></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-http-workers" class="text-input" min="0" title="Maximum number of workers" placeholder="Max workers..."></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-http-port" class="text-input" min="0" title="External port number" placeholder="External port..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-http-host" class="text-input" title="Host" placeholder="Host..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-http-paths" class="text-input" title="Paths: comma-separated list of paths" placeholder="Paths, e.g. first/path, second/path/here, third..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-rabbitmq-exchange" class="text-input" title="Exchange name" placeholder="Exchange name..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-rabbitmq-queue" class="text-input" title="Queue name" placeholder="Queue name..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-kinesis-key" class="text-input" title="Access key ID" placeholder="Access key ID..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-kinesis-secret" class="text-input" title="Secret access key" placeholder="Secret access key..."></li>' +
                    '<li class="triggers-field"><select id="triggers-kinesis-region" class="dropdown">' +
                        '<option value="">Select region...</option>' +
                        '<option value="us-east-2">us-east-2</option>' +
                        '<option value="us-east-1">us-east-1</option>' +
                        '<option value="us-west-1">us-west-1</option>' +
                        '<option value="us-west-2">us-west-2</option>' +
                        '<option value="ca-central-1">ca-central-1</option>' +
                        '<option value="ap-south-1">ap-south-1</option>' +
                        '<option value="ap-northeast-2">ap-northeast-2</option>' +
                        '<option value="ap-southeast-1">ap-southeast-1</option>' +
                        '<option value="ap-southeast-2">ap-southeast-2</option>' +
                        '<option value="ap-northeast-1">ap-northeast-1</option>' +
                        '<option value="eu-central-1">eu-central-1</option>' +
                        '<option value="eu-west-1">eu-west-1</option>' +
                        '<option value="eu-west-2">eu-west-2</option>' +
                        '<option value="sa-east-1">sa-east-1</option>' +
                    '</select></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-kinesis-stream" class="text-input" title="Stream name" placeholder="Stream name..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-kinesis-shards" class="text-input" title="Shards (e.g. 1,2-3,4)" placeholder="Shards, e.g. 1,2-3,4" pattern="\\s*\\d+(\\s*-\\s*\\d+)?(\\s*,\\s*\\d+(\\s*-\\s*\\d+)?)*(\\s*(,\\s*)?)?"></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-v3io-interval" class="text-input" min="0" title="Interval (ms)" placeholder="Interval (ms)..."></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-v3io-batch-size" class="text-input" min="0" title="Max batch size" placeholder="Max batch size..."></li>' +
                    '<li class="triggers-field"><input type="number" id="triggers-v3io-batch-wait" class="text-input" min="0" title="Max batch wait (ms)" placeholder="Max batch wait (ms)..."></li>' +
                    '<li class="triggers-field"><label><input type="checkbox" id="triggers-v3io-restart"> Restart</label></li>' +
                    '<li class="triggers-field"><label><input type="checkbox" id="triggers-v3io-incremental"> Incremental</label></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-v3io-attributes" class="text-input" title="Attributes (e.g. attr1, attr2)" placeholder="Attributes, e.g. attr1, attr2..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-v3io-queries" class="text-input" title="Queries (e.g. query1, query2)" placeholder="Queries, e.g. query1, query2..."></li>' +
                    '<li class="triggers-field"><input type="text" id="triggers-v3io-suffixes" class="text-input" title="Suffixes (e.g. suffix1, suffix2)" placeholder="Suffixes, e.g. suffix1, suffix2..."></li>' +
                    '</ul>')
                    .appendTo($('body')); // attaching to DOM temporarily in order to register event handlers

                $kind = $('#triggers-kind');
                $triggersFields = $('.triggers-field');

                $kind.change(function () {
                    var kind = $kind.val();
                    $triggersFields.each(function () {
                        var $field = $(this);
                        var id = $field.find('input,select,textarea').eq(0).prop('id');

                        // if kind is not empty, and the current field is associated with this kind
                        if (kind !== '' && _.find(properties, ['id', id]).kinds.includes(kind)) {
                            $field.show(0);
                        }
                        else {
                            clearInputs($field);
                            $field.hide(0);
                        }
                    });
                });

                $triggersFields.hide(0);
                $component.detach(); // detach from DOM so it keeps its state and it can be re-attached later
                return $component;
            },
            getValue: function () {
                var kind = $kind.val();
                var returnValue = {};

                properties
                    .filter(function (property) {
                        return property.kinds.includes(kind);
                    })
                    .forEach(function (property) {
                        var boolean = property.type === Boolean;
                        var $inputField = $('#' + property.id);
                        var inputValue = boolean ? $inputField.prop('checked') : $inputField.val();

                        inputValue = property.type(inputValue);

                        // if property is not a string nor an array, or if it is a non-empty string or array
                        if (![String, Array].includes(property.type) || !_(inputValue).isEmpty()) {
                            _.set(returnValue, property.path, boolean ? !inputValue : inputValue);
                        }
                    });

                returnValue.kind = kind;

                return returnValue;
            },
            isValueEmpty: function () {
                return getEmptyVisibleInputs().length > 0;
            },
            parseValue: function (value) {
                return _(properties)
                    .filter(function (property) {
                        return _.has(value, property.path);
                    })
                    .map(function (property) {
                        var displayValue = _.get(value, property.path);
                        return property.label + ': ' + (property.type === Boolean ? !displayValue : displayValue);
                    })
                    .join(' | ');
            },
            setFocusOnValue: function () {
                var $emptyInputs = getEmptyVisibleInputs();
                if ($emptyInputs.length > 0) {
                    // set focus on the first visible empty input field
                    $emptyInputs.eq(0).get(0).focus();
                }
            },
            clearValue: function () {
                clearInputs($component);
                $triggersFields.hide(0);
            }
        };

        /**
         * Gets all the text/number input fields that are empty
         * @returns {jQuery} a jQuery set of text/number input fields in the component that are empty
         *
         * @private
         */
        function getEmptyVisibleInputs() {
            return $component.find('input:not([type=checkbox]):visible,select:visible').filter(function () {
                return $(this).val() === '';
            });
        }

        /**
         * Converts a comma-delimited string of numbers and number ranges (X-Y) to an array of `Number`s
         * @param {string} ranges - a comma-separated string (might pad commas with spaces) consisting of either
         *     a single number, or two numbers with a hyphen between them, where the smaller number comes first (ranges
         *     where the first number is smaller than the second number will be ignored)
         * @returns {Array.<number>} an array of numbers representing all the numbers referenced in `ranges` param
         *
         * @private
         *
         * @example
         * toNumberArray('1,4-7,9-9,10')
         * // => [1, 4, 5, 6, 7, 9, 10]
         *
         * @example
         * toNumberArray('1, 2, 5-3, 9')
         * // => [1, 2, 9]
         *
         * @example
         * toNumberArray('   1  ,   2  ,  5   -   3   ,  9     ,       ')
         * // => [1, 2, 9]
         *
         * @example
         * toNumberArray('1, 2, 2, 3, 4, 4, 4, 4, 5, 5, 6, 1-2, 1-3, 1-4, 2-6, 3-4')
         * // => [1, 2, 3, 4, 5, 6]
         */
        function toNumberArray(ranges) {
            return _.chain(ranges)
                .replace(/\s+/g, '') // get rid of all white-space characters
                .trim(',') // get rid of leading and trailing commas
                .split(',') // get an array of strings, for each string that is between two comma delimiters
                .map(function (range) { // for each string - convert it to a number or an array of numbers
                    // if it is a sequence of digits - convert it to a `Number` value and return it
                    if (/^\d+$/g.test(range)) {
                        return Number(range);
                    }

                    // otherwise, attempt to parse it as a range (two sequences of digits delimited by a single hyphen)
                    var matches = range.match(/^(\d+)-(\d+)$/);

                    // attempt to convert both sequences of digits to `Number` values
                    var start   = Number(_.get(matches, '[1]'));
                    var end     = Number(_.get(matches, '[2]'));

                    // if any attempt above fails - return `null` to indicate a value that needs to be ignored later
                    // otherwise, return a range of `Number`s represented by that range (e.g. `'1-3'` is `[1, 2, 3]`)
                    return (Number.isNaN(start) || Number.isNaN(end) || start > end)
                        ? null
                        : _.range(start, end + 1);
                })
                .flatten() // make a single flat array (e.g. `[1, [2, 3], 4, [5, 6]]` becomes `[1, 2, 3, 4, 5, 6]`)
                .compact() // get rid of `null` values (e.g. `[null, 1, null, 2, 3, null]` becomes `[1, 2, 3]`)
                .uniq() // get rid of duplicate values (e.g. `[1, 2, 2, 3, 4, 4, 5]` becomes `[1, 2, 3, 4, 5]`)
                .sortBy() // sort the list in ascending order (e.g. `[4, 1, 5, 3, 2, 6]` becomes `[1, 2, 3, 4, 5, 6]`)
                .value();
        }

        /**
         * Splits a comma delimited string into an array of strings.
         * Delimiter could also be padded with spaces.
         * @param {string} string - the string to split
         * @returns {Array.<string>} a list of sub-string of `string`
         *
         * @private
         *
         * @example
         * toStringArray('a, b, c');
         * // => ['a', 'b', 'c']
         *
         * toStringArray('a , b  ,  c');
         * // => ['a', 'b', 'c']
         *
         * toStringArray('a b c');
         * // => ['a', 'b', 'c']
         *
         * toStringArray('');
         * // => []
         */
        function toStringArray(string) {
            return _.compact(string.split(/[\s,]+/g)); // in case `string` is empty: _.compact(['']) returns []
        }
    }

    var triggersInput = createKeyValuePairsInput('triggers', {}, 'name', 'attributes', createTriggersValueManipulator());

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
    var verticalSplitter = Split(['#upper', '#footer'], {
        sizes: [60, 40],
        minSize: [0, 0],
        gutterSize: SPLITTER_GUTTER_SIZE,
        snapOffset: SPLITTER_SNAP_OFFSET,
        direction: 'vertical',
        onDrag: _.debounce(emitWindowResize, SPLITTER_ON_DRAG_DEBOUNCE),
        onDragEnd: function () {
            var $handle = $('.gutter.gutter-vertical .collapse-handle');
            var size = verticalSplitter.getSizes()[1];
            if (size > 2 && $handle.hasClass('collapsed')) {
                $handle.removeClass('collapsed');
            }
            else if (size < 2 && !$handle.hasClass('collapsed')) {
                $handle.addClass('collapsed');
            }
        }
    });

    var horizontalSplitter = Split(['#editor-section', '#invoke-section'], {
        sizes: [60, 40],
        minSize: [0, 0],
        gutterSize: SPLITTER_GUTTER_SIZE,
        snapOffset: SPLITTER_SNAP_OFFSET,
        onDrag: _.debounce(emitWindowResize, SPLITTER_ON_DRAG_DEBOUNCE),
        onDragEnd: function () {
            var $handle = $('.gutter.gutter-horizontal .collapse-handle');
            var size = horizontalSplitter.getSizes()[1];
            if (size > 2 && $handle.hasClass('collapsed')) {
                $handle.removeClass('collapsed');
            }
            else if (size < 2 && !$handle.hasClass('collapsed')) {
                $handle.addClass('collapsed');
            }
        }
    });

    /**
     * Creates a click event handler for a collapse/expand button of a splitter
     * @param {Object} splitter - the splitter which collapse/expand need to be performed
     * @returns {function} a function that gets a click event and toggles collapsed/expanded state of splitter
     */
    function createCollapseExpandHandler(splitter) {
        return function (event) {
            event.preventDefault();
            event.stopPropagation();
            var $handle = $(this);
            if ($handle.hasClass('collapsed')) {
                $handle.removeClass('collapsed');
                splitter.setSizes([60, 40]);
            }
            else {
                $handle.addClass('collapsed');
                splitter.collapse(1);
            }
        };
    }

    // Create a collapse/expand button for horizontal splitter, register a click callback to it, and append it to gutter
    $('<i class="collapse-handle right"></i>')
        .click(createCollapseExpandHandler(horizontalSplitter))
        .appendTo($('.gutter.gutter-horizontal'));

    // Create a collapse/expand button for vertical splitter, register a click callback to it, and append it to gutter
    $('<i class="collapse-handle down"></i>')
        .click(createCollapseExpandHandler(verticalSplitter))
        .appendTo($('.gutter.gutter-vertical'));

    /* eslint-enable no-magic-numbers */
    /* eslint-enable new-cap */
});

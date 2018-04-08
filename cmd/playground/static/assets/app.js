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

    /**
     * Enum for key codes.
     * @readonly
     * @enum {number}
     */
    var KEY_CODES = {
        TAB: 9,
        ENTER: 13,
        ESC: 27,
        UP: 38,
        DOWN: 40
    };

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
        $element.find('select option:first-child').addBack('select option:first-child').prop('selected', true);
    }

    //
    // Web components (reusable components)
    //

    // ACE editor

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

    // Key-value list & input

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
            '<div class="new-pair-actions">' +
            '    <button class="new-pair-button pair-action button green" title="Add a new pair..." id="' + id + '-show-add-new-pair">+</button>' +
            '</div>' +
            '<div id="' + id + '-add-new-pair-form" class="add-new-pair-form">' +
            '    <div class="space-between">' +
            '        <div class="new-key"><input type="text" class="text-input new-key" id="' + id + '-new-key" placeholder="Type ' + headers.key + '..."></div>' +
            '        <div class="new-value">' + (_.isString(template) ? template : '') + '</div>' +
            '    </div>' +
            '    <div class="new-pair-actions">' +
            '        <button class="pair-action add-pair-apply-button button blue" title="Apply" id="' + id + '-add-new-pair-apply">&checkmark;</button>' +
            '        <button class="pair-action add-pair-cancel-button button grey" title="Cancel" id="' + id + '-add-new-pair-cancel">&times;</button>' +
            '    </div>' +
            '</div>'
        );

        var $pairList = $('#' + id + '-pair-list');
        var $newPairForm = $('#' + id + '-add-new-pair-form');
        var $newKeyInput = $('#' + id + '-new-key');
        var $newValueInput = $container.find('.new-value');
        var $showAddPairButton = $('#' + id + '-show-add-new-pair');
        var $newPairApplyButton = $('#' + id + '-add-new-pair-apply');
        var $newPairCancelButton = $('#' + id + '-add-new-pair-cancel');

        $showAddPairButton.click(showAddNewPair);
        $newPairCancelButton.click(hideAddNewPairForm);
        $newPairApplyButton.click(addNewPair);

        if (template instanceof jQuery) {
            template.appendTo($newValueInput);
        }

        $newPairForm.hide(0);

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
         *
         * @private
         */
        function clearInput() {
            vManipulator.clearValue();
            $newKeyInput.val('').get(0).focus();
        }

        /**
         * Shows the add new pair form (and hides the "+" button) and puts focus on the new key text box
         *
         * @private
         */
        function showAddNewPair() {
            $showAddPairButton.hide(0);
            $newPairForm.show(0);
            $newKeyInput.get(0).focus();
        }

        /**
         * Clears and hides the "add new pair" form and put focus on the "add new pair" button
         *
         * @private
         */
        function hideAddNewPairForm() {
            clearInput();
            $newPairForm.hide(0);
            $showAddPairButton.show(0);
            $showAddPairButton.get(0).focus();
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
                hideAddNewPairForm();
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
            $pairList.find('[class=remove-pair-button]').off('click');

            // remove all DOM of pair list
            $pairList.empty();

            // if there are currently no pairs on the list - display an appropriate message
            if (_(pairs).isEmpty()) {
                $pairList.append('<li>Empty list. You may add new entries.</li>');
            }

            // otherwise - build HTML for list of key-value pairs, plus add headers
            else {
                $pairList.append('<li class="pair-list-item space-between">' + _(pairs).map(function (value, key) {
                    return '<span class="pair-key text-ellipsis" title="' + key + '">' + key + '</span>' +
                        '<span class="pair-value text-ellipsis" title="' + vManipulator.parseValue(value) + '">' +
                        vManipulator.parseValue(value) + '</span>';
                }).join('</li><li class="pair-list-item space-between">') + '</li>');

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

    /**
     * Creates a customized value manipulator to use with the key-value input component. It receives a list of field
     * descriptors and optionally a field descriptor for a "kind" drop-down menu. Each field descriptor describes how
     * each field should be rendered (its label to display on key-value list, its type and type-specific attributes,
     * the model property bound to this field, which kinds from kind list are associated with it, if any, and more).
     * If a proper "kind" field descriptor is provided, it is rendered as a drop-down menu. On selecting an option on
     * this menu, a different set of fields is displayed (according to the association set in each field descriptor).
     * If a field is not associated to any kind, it will always be displayed. These fields will be displayed above the
     * drop-down menu of kind. The rest of the fields (associated to at least one kind) will be displayed below it.
     *
     * @param {Array.<Object>} fields - outlines the fields constructing the _value_ of the key-value pair.
     * @param {string} [fields[].label] - the label to display for this field in the list of key-value pairs.
     *      Omit it if this field's value should not be displayed on key-value list.
     * @param {string} [fields[].path] - dot-delimited path to property in object model to populate by this field.
     *      Omit it if this field's value should not be stored in model.
     * @param {string} [fields[].id] - HTML element's "id" attribute for this field.
     * @param {string} [fields[].type='string'] - the type of the field. must be one of:
     *     `'boolean'` - will be rendered as a checkbox, will set model to boolean values
     *     `'number'` - will be rendered as number input field, will set model to number values
     *     `'string'` - will be rendered as text input field, will set model to string values
     *     `'dropdown'` - will be rendered as drop-down menu field, will set string values in model
     *     `'numberArray'` - will be rendered as text input field that expects number ranges (e.g. "1-2, 3,4,5-6 ,7")
     *                       will set model to array of number values (e.g. `[1, 2, 3, 4, 5, 6, 7]`)
     *     `'stringArray'` - will be rendered as for text input field that expects comma-delimited list of strings
     *                       (e.g. "abc, def, ghi"), will set model to array of strings (e.g. `['abc', 'def', 'ghi']`)
     * @param {boolean} [fields[].required=false] - determines whether or not the field is required (i.e. cannot be
     *     left empty). If this property exists and is equal to `true` then the field is required, otherwise (if it is
     *     either omitted or exists and equals to `false`) the field is optional, i.e., can be left empty. (irrelevant
     *     to checkboxes).
     * @param {string} [fields[].title=''] - the tooltip hint to display on hovering the field.
     * @param {string} [fields[].checkedValue=true] - the model value when checked (for checkboxes only).
     * @param {string} [fields[].uncheckedValue=false] - the model value when unchecked (for checkboxes only).
     * @param {string} [fields[].checkedLabel='True'] - the view value (label) when checked (for checkboxes only).
     * @param {string} [fields[].uncheckedLabel='False'] - the view value (label) when unchecked (for checkboxes only).
     * @param {string} [fields[].placeholder=''] - the placeholder text to use in text/number boxes (for text/number
     *     boxes only).
     * @param {string} [fields[].min] - the minimum allowed input number (for number boxes only).
     * @param {string} [fields[].max] - the maximum allowed input number (for number boxes only).
     * @param {string} [fields[].step] - the step size allowed for input number (for number boxes only).
     * @param {string} [fields[].maxlength] - the maximum number of characters allowed (for text boxes only).
     * @param {string} [fields[].pattern=''] - the RegEx validation pattern to use (for text boxes only).
     * @param {Array.<{value: string, label: string}>} [fields[].options] - the values and their corresponding labels
     *     to display as options of drop-down menu (for drop-down menus only).
     * @param {Array.<string>} [fields[].kinds] - list of kinds this field is associated to. when one of the kind values
     *     on this list is selected in the kind drop-down, this field will be visible and relevant.
     * @param {Object} kindField - a field descriptor for a drop-down menu input field with a list of available kinds.
     * @param {string} [delimiter='|'] the delimiter to use in key-value list items, between different field values.
     * @returns {{getTemplate: function, getValue: function, isValueEmpty: function, parseValue: function,
     *     setFocusOnValue: function, clearValue: function}} returns the required methods for value manipulator.
     */
    function createCustomValueManipulator(fields, kindField, delimiter) {
        var $component = null;
        var fieldDescriptors = _.chain(fields)
            .defaultTo([])
            .cloneDeep()                   // clone the provided fields array so it won't be changed by this function
            .map(function (field) {        // then for each field ..
                return _.defaults(field, { // .. assign default properties that will be used later in the logic
                    kinds: [],
                    $input: null
                });
            })
            .value();

        var renderers = {
            dropdown: function (fieldDescriptor) {
                return $('<select></select>', {
                    'class': 'dropdown',
                    'title': fieldDescriptor.title,
                    'required': fieldDescriptor.required
                })
                    .append(fieldDescriptor.options
                        .filter(function (option) {
                            return _.isString(option.value) && _.isString(option.label) && !_.isEmpty(option.label);
                        })
                        .map(function (option) {
                            return $('<option></option>', { value: option.value }).text(option.label);
                        }));
            },
            string: function (fieldDescriptor) {
                return $('<input>', _.extend({
                    'type': 'text',
                    'class': 'text-input'
                }, _.pick(fieldDescriptor, ['required', 'title', 'placeholder', 'pattern', 'maxlength'])));
            },
            number: function (fieldDescriptor) {
                return $('<input>', _.extend({
                    'type': 'number',
                    'class': 'text-input'
                }, _.pick(fieldDescriptor, ['required', 'title', 'placeholder', 'max', 'min', 'step'])));
            },
            boolean: function (fieldDescriptor) {
                return $('<input>', {
                    type: 'checkbox',
                    id: fieldDescriptor.id
                }).prependTo($('<label></label>', { title: fieldDescriptor.title }).text(' ' + fieldDescriptor.label));
            }
        };

        var kindDescriptor = _.cloneDeep(_.defaultTo(kindField, null));
        var $kindInput = null;

        return {
            getTemplate: function () {
                // create the component's field list
                $component = $('<ul></ul>', { 'class': 'custom-key-value-list' });

                // if "kind" drop-down menu input field descriptor was provided and its option list is non-empty
                if (kindDescriptor !== null &&
                    kindDescriptor.type === 'dropdown' && !_(kindDescriptor.options).isEmpty()) {
                    // render it as a drop-down menu DOM element
                    $kindInput = renderers.dropdown(kindDescriptor)

                        // append it to the general component
                        .appendTo($('<li></li>').appendTo($component))

                        // register a "change"-event handler to it so each field that belongs to some kind will
                        // show/hide according to the selected kind
                        .change(function () {
                            var kind = $kindInput.val();
                            fieldDescriptors
                                .filter(function (fieldDescriptor) {
                                    return !_(fieldDescriptor.kinds).isEmpty();
                                })
                                .forEach(function (fieldDescriptor) {
                                    clearInputs(fieldDescriptor.$input);
                                    if (fieldDescriptor.kinds.includes(kind)) {
                                        fieldDescriptor.$input.closest('li').show(0);
                                    }
                                    else {
                                        fieldDescriptor.$input.closest('li').hide(0);
                                    }
                                });
                        });
                }

                // for each field descriptor - render the field and append it to component's field list
                fieldDescriptors.forEach(function (fieldDescriptor) {
                    // pick appropriate renderer according to field type, default to text-box
                    var renderer = _(renderers[fieldDescriptor.type]).defaultTo(renderers.string);

                    // create the DOM field element
                    var $fieldElement = renderer(fieldDescriptor);

                    // get the DOM element that needs to be appended to the component's list:
                    // if the field is composed of nested DOM elements (for example a <label> element wrapping a
                    // <input type="checkbox"> element) then get its outer-most one (the <label> in our example),
                    // otherwise get the DOM element of the input field itself
                    var $fieldWrapper = $fieldElement.parents(':not(:empty)').addBack().first();

                    // create a new list item and append above element to it
                    var $listItem = $('<li></li>', { 'class': 'triggers-field' }).append($fieldWrapper);

                    // append this field to the DOM-tree according to following logic:
                    // if there is no "kind" drop-down menu - just append to end list
                    // if there is a "kind" drop-down menu and field belongs to no kind – append field before "kind"
                    // if there is a "kind" drop-down menu and field belongs to some kind - append to end of list
                    if ($kindInput === null) {
                        $component.append($listItem);
                    }
                    else if (_(fieldDescriptor.kinds).isEmpty()) {
                        $kindInput.closest('li').before($listItem);
                    }
                    else {
                        $component.append($listItem);

                        if (_(kindDescriptor.options).size() > 1) {
                            $listItem.hide(0);
                        }
                    }

                    // store field element in field descriptor for later easy access
                    fieldDescriptor.$input = $fieldElement;
                });

                return $component;
            },
            getValue: function () {
                var kind = kindDescriptor === null ? null : $kindInput.val();
                var returnValue = {};

                fieldDescriptors

                    // filter out fields that are not of the selected kind (fields belonging to no kind are filtered in)
                    // also filter out fields with no `path` property
                    .filter(function (fieldDescriptor) {
                        return _(fieldDescriptor).has('path') && (  // this field should be stored in model at some path
                            kind === null ||                        // and there is no "kind" drop-down menu, or:
                            _(fieldDescriptor.kinds).isEmpty() ||   // this field belongs to no kind, or:
                            _(fieldDescriptor.kinds).includes(kind) // this field belongs to the selected kind
                        );
                    })

                    // use each field's contribution to populating model object
                    .forEach(function (fieldDescriptor) {
                        // extract the field's view-value from the DOM element
                        var viewValue = fieldDescriptor.$input.prop('type') === 'checkbox'
                            ? fieldDescriptor.$input.prop('checked')
                            : fieldDescriptor.$input.val();

                        // convert the view-value to appropriate model-value using appropriate conversion
                        var modelValue = toModelValue(viewValue, fieldDescriptor);

                        // assign model-value to model at path
                        _.set(returnValue, fieldDescriptor.path, modelValue);
                    });

                if (_(kindDescriptor).has('path')) {
                    _.set(returnValue, kindDescriptor.path, kind);
                }

                return returnValue;
            },
            isValueEmpty: function () {
                return getRequiredEmptyVisibleInputs().length > 0;
            },
            parseValue: function (value) {
                // for each field ..
                return _(fieldDescriptors)

                // .. and "kind" field, if provided
                    .concat(kindDescriptor === null ? [] : [kindDescriptor])

                    // take only the fields that are saved in model and that have a 'label' property
                    .filter(function (fieldDescriptor) {
                        return _(fieldDescriptor).has('label') && _(value).has(fieldDescriptor.path);
                    })

                    // and extract their model-value and convert it to view-value
                    .map(function (fieldDescriptor) {
                        var modelValue = _(value).get(fieldDescriptor.path);
                        var viewValue = toViewValue(modelValue, fieldDescriptor);
                        return fieldDescriptor.label + ': ' + viewValue;
                    })

                    // concatenate all view-values to a single string of delimited label-value list
                    .join(' ' + _(delimiter).defaultTo('|') + ' ');
            },
            setFocusOnValue: function () {
                var $emptyInputs = getRequiredEmptyVisibleInputs();
                if ($emptyInputs.length > 0) {
                    // set focus on the first visible empty input field
                    $emptyInputs.eq(0).get(0).focus();
                }
            },
            clearValue: function () {
                clearInputs($component);
                if (_(kindDescriptor.options).size() > 1) {
                    fieldDescriptors
                        .filter(function (fieldDescriptor) {
                            return !_(fieldDescriptor.kinds).isEmpty();
                        })
                        .forEach(function (fieldDescriptor) {
                            fieldDescriptor.$input.closest('li').hide();
                        });
                }
            }
        };

        /**
         * Gets all the text/number/drop-down input fields that are required but empty and visible
         * @returns {jQuery} a jQuery set of text/number/drop-down input fields nested in the component that are
         *     required, empty and visible
         *
         * @private
         */
        function getRequiredEmptyVisibleInputs() {
            return $component
                .find('input:not([type=checkbox])[required]:visible,select[required]:visible')
                .filter(function () {
                    return $(this).val() === '';
                });
        }

        /**
         * Converts a field's model-value to view-value for display in key-value list.
         * @param {*} modelValue - the value that is currently stored in model.
         * @param {Object} fieldDescriptor - the relevant field descriptor (determines some values for conversions).
         * @returns {string} the view-value for the provided model-value
         *
         * @private
         */
        function toViewValue(modelValue, fieldDescriptor) {
            // maps between a field's type (determined in `fieldDescriptor.type`) and its relevant conversion function
            // types not listed as keys in this object will be not be converted, but instead passed as-is
            var converters = {
                boolean: fromBoolean,
                dropdown: fromDropdown
            };

            var convert = _(converters[fieldDescriptor.type]).defaultTo(_.identity);
            return convert(modelValue);

            /**
             * Returns the appropriate string according to the current model-value of the boolean/checkbox field value.
             * @param {*} value - the model-value of a checkbox field to convert to its appropriate view-value.
             * @returns {string} `checkedLabel` if `modelValue` equals to `checkedValue`, or `uncheckedLabel` otherwise
             *
             * @private
             */
            function fromBoolean(value) {
                return value === _(fieldDescriptor.checkedValue).defaultTo(true)
                    ? _(fieldDescriptor.checkedLabel).defaultTo('True')
                    : _(fieldDescriptor.uncheckedLabel).defaultTo('False');
            }

            /**
             * Returns the label of the selected option of a drop-down menu field which is this field's model-value.
             * @param {*} value - the model-value of a drop-down menu field to convert to its appropriate view-value.
             * @returns {string} the label corresponding to the model-value (the currently selected option)
             *
             * @private
             */
            function fromDropdown(value) {
                return _.get(_.find(fieldDescriptor.options, { value: value }), 'label');
            }
        }

        /**
         * Converts a field's view-value to model-value.
         * @param {string} viewValue - the view-value retrieved from DOM element of input field
         * @param {Object} fieldDescriptor - the relevant field descriptor (determines some values for conversions).
         * @returns {*} the model-value for the provided view-value
         *
         * @private
         */
        function toModelValue(viewValue, fieldDescriptor) {
            // maps between a field's type (determined in `fieldDescriptor.type`) and its relevant conversion function
            // types not listed as keys in this object will be not be converted, but instead passed as-is
            var converters = {
                number: _.toNumber,
                boolean: toBoolean,
                numberArray: toNumberArray,
                stringArray: toStringArray
            };

            var convert = _(converters[fieldDescriptor.type]).defaultTo(_.identity);
            return convert(viewValue);

            /**
             * Converts a comma-delimited string of numbers and number ranges (X-Y) to an array of `Number`s
             * @param {string} ranges - a comma-separated string (might pad commas with spaces) consisting of either
             *     a single number, or two numbers with a hyphen between them, where the smaller number comes first
             *     (ranges where the first number is smaller than the second number will be ignored)
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

                        // otherwise, attempt to parse it as a range of numbers (two sequences of digits delimited by a
                        // single hyphen)
                        var matches = range.match(/^(\d+)-(\d+)$/);

                        // attempt to convert both sequences of digits to `Number` values
                        var start   = Number(_.get(matches, '[1]'));
                        var end     = Number(_.get(matches, '[2]'));

                        // if any attempt above fails - return `null` to indicate a value that needs to be ignored later
                        // otherwise, return a range of `Number`s represented by that range
                        // (e.g. `'1-3'` is `[1, 2, 3]`)
                        return (Number.isNaN(start) || Number.isNaN(end) || start > end)
                            ? null
                            : _.range(start, end + 1);
                    })
                    .flatten() // make a single flat array (e.g. `[1, [2, 3], 4, [5, 6]]` to `[1, 2, 3, 4, 5, 6]`)
                    .without(false, null, '', undefined, NaN) // get rid of `null` values (e.g. `[null, 1, null, 2, 3, null]` to `[1, 2, 3]`)
                    .uniq() // get rid of duplicate values (e.g. `[1, 2, 2, 3, 4, 4, 5]` to `[1, 2, 3, 4, 5]`)
                    .sortBy() // sort the list in ascending order (e.g. `[4, 1, 5, 3, 2, 6]` to`[1, 2, 3, 4, 5, 6]`)
                    .value();
            }

            /**
             * Splits a comma delimited string into an array of strings.
             * Delimiter could also be padded with spaces.
             * @param {string} stringList - the comma-separated string to split.
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
            function toStringArray(stringList) {
                return _.compact(stringList.split(/[\s,]+/g)); // in case `string` is empty: _.compact(['']) returns []
            }

            /**
             * Returns `checkedValue` if `value` is `true`, or `uncheckedValue` if `value` is `false`
             * @param {boolean} checkboxValue - its value determines whether to return `checkedValue` or
             *     `uncheckedValue`.
             * @returns {*} `checkedValue` if `value` is `true`, or `uncheckedValue` otherwise
             *
             * @private
             *
             * @example
             * toBoolean(true);
             * // => true
             *
             * toBoolean(false);
             * // => false
             *
             * toBoolean(true, false, true);
             * // => false
             *
             * toBoolean(false, false, true);
             * // => true
             *
             * toBoolean(NaN, 1);
             * // => false
             *
             * toBoolean('hello', 1, '2');
             * // => 1
             *
             * toBoolean(null, NaN, Date.now());
             * // => 1516204876624
             */
            function toBoolean(checkboxValue) {
                return checkboxValue
                    ? _.defaultTo(fieldDescriptor.checkedValue, true)
                    : _.defaultTo(fieldDescriptor.uncheckedValue, false);
            }
        }
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
        'dotnetcore': {
            extension: 'cs',
            label: '.NET Core'
        },
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
        },
	'java': {
	    extension: 'java',
	    label: 'Java'
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
                        clearAll();
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
        var $options = $functionListItems.children(':visible');
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

                // test if this is an exact match (omitting the runtime, taking the name only)
                if ($element.text().replace(/\s+\(\w+\)/, '') === inputValue) {
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
    $functionsFilterBox.keyup(_.debounce(function () {
        convertInputToLowerCase($functionsFilterBox);
        updateFunctionFilter();
    }, FILTER_BOX_KEY_UP_DEBOUNCE));

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
        createNewFunction(name.toLowerCase(), runtime);
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
        clearInputs($('#invoke-section-wrapper'));
        setInvokeBodyField();
        inputBodyEditor.setText('');
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
     * Converts the value of a text input box to all lowercase letters
     * @param {jQuery|HTMLElement} $inputBox - the <input type="text"> element whose value to convert
     */
    function convertInputToLowerCase($inputBox) {
        $($inputBox).val($($inputBox).val().toLowerCase());
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
                var httpPort             = _.get(selectedFunction, 'status.httpPort', 0);
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
                    $('#commands').val(commands.join('\n'));
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
            createNewFunction(name.toLowerCase(), runtime);
            $createNewPopUp.hide(0);
        }
    });

    // Register click event handler for close button to close pop-up
    $('#create-new-close').click(function () {
        $createNewPopUp.hide(0);
    });

    // Register key-up event handler for function name box in "Create new" pop-up
    $createNewName.keyup(_.debounce(function () {
        convertInputToLowerCase($createNewName);
    }, FILTER_BOX_KEY_UP_DEBOUNCE));

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
            _.assign(selectedFunction.metadata, {
                labels: configLabels.getKeyValuePairs(),
                namespace: $('#namespace').val(),
                name: name
            });

            _.assign(selectedFunction.spec, {
                build: _.assign(_.get(selectedFunction, 'spec.build', {}), {
                    baseImageName: $('#base-image').val(),
                    commands: _.without($('#commands').val().replace('\r', '\n').split('\n'), ''),
                    path: path,
                    registry: ''
                }),
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
            });

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
                .fail(function (jqXHR) {
                    switch (jqXHR.status) {
                        case 429: // eslint-disable-line no-magic-numbers
                            showErrorToast('Deploy failed - another function currently deploying');
                            break;
                        default:
                            showErrorToast('Deploy failed... (' + jqXHR.responseText + ')');
                    }
                });
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
        var path = '/' + _.trimStart($inputPath.val(), '/ ');
        var url = workingUrl + '/invocations';
        var method = $('#input-method').val();
        var body = isFileInput ? $invokeFile.get(0).files.item(0) : inputBodyEditor.getText();
        var contentType = isFileInput ? body.type : $inputContentType.val();
        var dataType = isFileInput ? 'binary' : 'text';
        var level = $('#input-level').val();
        var output = '';

        $.ajax(url, {
            method: method,
            data: body,
            dataType: dataType,
            cache: false,
            contentType: contentType,
            processData: false,
            beforeSend: function (xhr) {
                xhr.setRequestHeader('x-nuclio-path', path);
                xhr.setRequestHeader('x-nuclio-function-name', _.get(selectedFunction, 'metadata.name'));
                xhr.setRequestHeader('x-nuclio-function-namespace', _.get(selectedFunction, 'metadata.namespace', 'default'));
                xhr.setRequestHeader('x-nuclio-log-level', level);
            }
        })
            .done(function (data, textStatus, jqXHR) {
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
         * Prints to log the function invocation output, logs and response details
         * @param {Object} jqXHR - the jQuery XHR object
         */
        function printToLog(jqXHR) {
            var emptyMessage = '&lt;empty&gt;';
            var logs = [];

            // parse logs from "x-nuclio-logs" response header
            var logsString = extractResponseHeader(jqXHR.getAllResponseHeaders(), 'x-nuclio-logs', '[]');

            try {
                logs = JSON.parse(logsString);
            }
            catch (error) {
                console.error('Error parsing "x-nuclio-logs" response header as a JSON:\n' + error.message);
                logs = [];
            }

            // add function invocation log entry consisting of response status, headers adn body
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
    var validFileExtensions = ['.cs', '.py', '.pypy', '.go', '.sh', '.txt'];

    var codeEditor = createEditor('code-editor', 'text', true, true, false, CODE_EDITOR_MARGIN);

    var $codeEditor = $('#code-editor');
    var $codeEditorDropZone = $('#code-editor-drop-zone');

    // Register event handlers for drag'n'drop of files to code editor
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
    // "Code" tab - "Invoke" pane
    //

    var inputBodyEditor = createEditor('input-body-editor', 'json', false, false, false, 0);

    var $invokePaneElements = $('#invoke-section').find('select, input, button');
    var $invokeInputBody = $('#input-body-editor');
    var $invokeFile = $('#input-file');
    var $inputPath = $('#input-path');
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
    $inputContentType.change(setInvokeBodyField);

    /**
     * Displays either a text editor or a file input field according to selected option of Content Type drop-down list
     */
    function setInvokeBodyField() {
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
    }

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
        var httpPort = _.get(selectedFunction, 'status.httpPort', 0);
        $('#input-url').html(hide ? '' : loadedUrl.get('protocol', 'hostname') + ':' + httpPort);
    }

    // initially disable all controls
    disableInvokePane(true);

    //
    // "Code" tab - "Log" pane
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
    // "Code" tab - "Log" pane - Polling
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
                    else if (_.get(pollResult, 'status.state') === 'ready') {
                        if (selectedFunction === null) {
                            selectedFunction = {};
                        }

                        // store the port for newly created function
                        var httpPort = _.get(pollResult, 'status.httpPort', 0);
                        _.set(selectedFunction, 'status.httpPort', httpPort);

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
        return !['ready', 'error'].includes(firstWord);
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
    var dataBindingClasses = {
        id: 'config-data-bindings-class',
        label: 'Class',
        path: 'class',
        type: 'dropdown',
        title: 'Select a data binding class to show its relevant fields',
        required: true,
        options: [{ value: 'v3io', label: 'v3io' }]
    };
    var dataBindingFields = [
        {
            id: 'config-data-bindings-url',
            label: 'URL',
            path: 'url',
            type: 'string',
            placeholder: 'URL (e.g. http://12.34.56.78:9999/path)',
            title: 'Enter v3io URL',
            required: true,
            kinds: ['v3io']
        },
        {
            id: 'config-data-bindings-secret',
            label: 'Secret',
            path: 'secret',
            type: 'string',
            placeholder: 'Secret...',
            title: 'Enter v3io secret',
            required: true,
            kinds: ['v3io']
        }
    ];
    var configDataBindings = createKeyValuePairsInput('config-data-bindings', {}, 'name', 'attributes',
        createCustomValueManipulator(dataBindingFields, dataBindingClasses));

    //
    // "Triggers" tab
    //

    var triggerKinds = {
        type: 'dropdown',
        path: 'kind',
        label: 'Kind',
        title: 'Pick trigger kind to show/hide relevant fields to that kind',
        require: true,
        options: [
            {
                value: '',
                label: 'Select kind...'
            },
            {
                value: 'eventhub',
                label: 'Azure Event Hub'
            },
            {
                value: 'http',
                label: 'HTTP'
            },
            {
                value: 'rabbit-mq',
                label: 'RabbitMQ'
            },
            {
                value: 'kafka',
                label: 'Kafka'
            },
            {
                value: 'kinesis',
                label: 'Kinesis'
            },
            {
                value: 'nats',
                label: 'NATS'
            },
            {
                value: 'v3ioItemPoller',
                label: 'v3io item poller'
            },
            {
                value: 'v3ioStream',
                label: 'v3io stream'
            },
            {
                value: 'cron',
                label: 'cron'
            }
        ]
    };

    var triggerFields = [
        {
            id: 'triggers-enabled',
            path: 'disabled',
            type: 'boolean',
            label: 'Enabled',
            title: 'Enable/disable trigger',
            checkedValue: false,
            uncheckedValue: true
        },
        {
            id: 'triggers-url',
            path: 'url',
            type: 'string',
            label: 'URL',
            title: 'URL',
            placeholder: 'URL, e.g. http://12.34.56.78:9999/path',
            kinds: ['kafka', 'nats', 'v3ioItemPoller', 'v3ioStream']
        },
        {
            id: 'triggers-total',
            path: 'numPartitions',
            type: 'number',
            label: 'Total',
            title: 'Total number of partitions/shards',
            placeholder: 'Total partitions/shards...',
            required: true,
            min: 0,
            kinds: ['eventhub', 'kafka', 'kinesis', 'v3ioItemPoller', 'v3ioStream']
        },
        {
            id: 'triggers-partitions',
            path: 'attributes.partitions',
            type: 'numberArray',
            label: 'Partitions',
            title: 'Partitions (e.g. 1,2-3,4)',
            placeholder: 'Partitions, e.g. 1,2-3,4',
            pattern: '\\s*\\d+(\\s*-\\s*\\d+)?(\\s*,\\s*\\d+(\\s*-\\s*\\d+)?)*(\\s*(,\\s*)?)?',
            kinds: ['kafka', 'v3ioItemPoller', 'v3ioStream', 'eventhub']
        },
        {
            id: 'triggers-topic',
            path: 'attributes.topic',
            type: 'string',
            label: 'Topic',
            title: 'Topic',
            placeholder: 'Topic...',
            required: true,
            kinds: ['kafka', 'nats']
        },

        // http specific
        {
            id: 'triggers-http-workers',
            path: 'maxWorkers',
            type: 'number',
            label: 'Max workers',
            required: true,
            min: 0,
            title: 'Maximum number of workers',
            placeholder: 'Max workers, e.g. 4',
            kinds: ['http']
        },
        {
            id: 'triggers-http-paths',
            path: 'attributes.ingresses.http.paths',
            type: 'stringArray',
            label: 'Paths',
            title: 'Paths: comma-separated list of paths',
            placeholder: 'Paths, e.g. first/path, second/path/here, third...',
            kinds: ['http']
        },
        {
            id: 'triggers-http-host',
            path: 'attributes.ingresses.http.host',
            type: 'string',
            label: 'Host',
            title: 'Host',
            placeholder: 'Host...',
            kinds: ['http']
        },
        {
            id: 'triggers-http-port',
            path: 'attributes.port',
            type: 'number',
            label: 'External port',
            min: 0,
            title: 'External port number',
            placeholder: 'Port, e.g. 5326',
            kinds: ['http']
        },

        // RabbitMQ specific
        {
            id: 'triggers-rabbitmq-exchange',
            path: 'attributes.exchangeName',
            type: 'string',
            label: 'Exchange name',
            title: 'Exchange name',
            placeholder: 'Exchange name...',
            required: true,
            kinds: ['rabbit-mq']
        },
        {
            id: 'triggers-rabbitmq-queue',
            path: 'attributes.queueName',
            type: 'string',
            label: 'Queue name',
            title: 'Queue name',
            placeholder: 'Queue name...',
            required: true,
            kinds: ['rabbit-mq']
        },

        // Kinesis specific
        {
            id: 'triggers-kinesis-key',
            path: 'attributes.accessKeyID',
            type: 'string',
            label: 'Access key ID',
            title: 'Access key ID',
            placeholder: 'Access key ID...',
            required: true,
            kinds: ['kinesis']
        },
        {
            id: 'triggers-kinesis-secret',
            path: 'attributes.secretAccessKey',
            type: 'string',
            label: 'Secret access key',
            title: 'Secret access key',
            placeholder: 'Secret access key...',
            required: true,
            kinds: ['kinesis']
        },
        {
            id: 'triggers-kinesis-region',
            path: 'attributes.regionName',
            type: 'dropdown',
            label: 'Region',
            options: [
                { value: '', label: 'Select region...' },
                { value: 'us-east-2', label: 'us-east-2' },
                { value: 'us-east-1', label: 'us-east-1' },
                { value: 'us-west-1', label: 'us-west-1' },
                { value: 'us-west-2', label: 'us-west-2' },
                { value: 'ca-central-1', label: 'ca-central-1' },
                { value: 'ap-south-1', label: 'ap-south-1' },
                { value: 'ap-northeast-2', label: 'ap-northeast-2' },
                { value: 'ap-southeast-1', label: 'ap-southeast-1' },
                { value: 'ap-southeast-2', label: 'ap-southeast-2' },
                { value: 'ap-northeast-1', label: 'ap-northeast-1' },
                { value: 'eu-central-1', label: 'eu-central-1' },
                { value: 'eu-west-1', label: 'eu-west-1' },
                { value: 'eu-west-2', label: 'eu-west-2' },
                { value: 'sa-east-1', label: 'sa-east-1' }
            ],
            required: true,
            kinds: ['kinesis']
        },
        {
            id: 'triggers-kinesis-stream',
            path: 'attributes.streamName',
            type: 'string',
            label: 'Stream',
            title: 'Stream name',
            placeholder: 'Stream name...',
            required: true,
            kinds: ['kinesis']
        },
        {
            id: 'triggers-kinesis-shards',
            path: 'attributes.shards',
            type: 'numberArray',
            label: 'Shards',
            title: 'Shards (e.g. 1,2-3,4)',
            placeholder: 'Shards, e.g. 1,2-3,4',
            pattern: '\\s*\\d+(\\s*-\\s*\\d+)?(\\s*,\\s*\\d+(\\s*-\\s*\\d+)?)*(\\s*(,\\s*)?)?',
            required: true,
            kinds: ['kinesis']
        },

        // v3io Item Poller specific
        {
            id: 'triggers-v3io-interval',
            path: 'attributes.intervalMs',
            type: 'number',
            label: 'Interval (ms)',
            title: 'Interval (ms)',
            placeholder: 'Interval (ms)...',
            min: 0,
            required: true,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-batch-size',
            path: 'attributes.maxBatchSize',
            type: 'number',
            label: 'Max Batch Size',
            title: 'Max batch size',
            placeholder: 'Max batch size...',
            required: true,
            min: 0,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-batch-wait',
            path: 'attributes.maxBatchWaitMs',
            type: 'number',
            label: 'Max Batch Wait (ms)',
            title: 'Max batch wait (ms)',
            placeholder: 'Max batch wait (ms)...',
            required: true,
            min: 0,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-restart',
            path: 'attributes.restart',
            type: 'boolean',
            label: 'Restart',
            title: 'Restart',
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-incremental',
            path: 'attributes.incremental',
            type: 'boolean',
            label: 'Incremental',
            title: 'Incremental',
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-attributes',
            path: 'attributes.attributes',
            type: 'stringArray',
            label: 'Attributes',
            title: 'Attributes (e.g. attr1, attr2)',
            placeholder: 'Attributes, e.g. attr1, attr2...',
            required: true,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-queries',
            path: 'attributes.queries',
            type: 'stringArray',
            label: 'Queries',
            title: 'Queries (e.g. query1, query2)',
            placeholder: 'Queries, e.g. query1, query2...',
            required: true,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-suffixes',
            path: 'attributes.suffixes',
            type: 'stringArray',
            label: 'Suffixes',
            title: 'Suffixes (e.g. suffix1, suffix2)',
            placeholder: 'Suffixes, e.g. suffix1, suffix2...',
            required: true,
            kinds: ['v3ioItemPoller']
        },
        {
            id: 'triggers-v3io-paths',
            path: 'paths',
            type: 'stringArray',
            label: 'Paths',
            title: 'Paths (e.g. path1, path2)',
            placeholder: 'Paths, e.g. path1, path2...',
            required: true,
            kinds: ['v3ioItemPoller']
        },

        // Azure eventhub specific
        {
            id: 'triggers-eventhub-key',
            path: 'attributes.sharedAccessKeyName',
            type: 'string',
            label: 'Shared Access Key Name',
            title: 'Shared Access Key Name',
            placeholder: 'Shared Access Key Name...',
            required: true,
            kinds: ['eventhub']
        },
        {
            id: 'triggers-eventhub-key-value',
            path: 'attributes.sharedAccessKeyValue',
            type: 'string',
            label: 'Shared Access Key Value',
            title: 'Shared Access Key Value',
            placeholder: 'Shared Access Key Value...',
            required: true,
            kinds: ['eventhub']
        },
        {
            id: 'triggers-eventhub-namespace',
            path: 'attributes.namespace',
            type: 'string',
            label: 'Namespace',
            title: 'Namespace',
            placeholder: 'Namespace...',
            required: true,
            kinds: ['eventhub']
        },
        {
            id: 'triggers-eventhub-eventhubname',
            path: 'attributes.eventHubName',
            type: 'string',
            label: 'Event Hub name',
            title: 'Event Hub name',
            placeholder: 'Event Hub name...',
            required: true,
            kinds: ['eventhub']
        },
        {
            id: 'triggers-eventhub-consumergroup',
            path: 'attributes.consumerGroup',
            type: 'string',
            label: 'Consumer Group',
            title: 'Consumer Group',
            placeholder: 'Consumer Group',
            required: true,
            kinds: ['eventhub']
        },

        // cron specific
        {
            id: 'triggers-cron-schedule',
            path: 'attributes.schedule',
            type: 'string',
            label: 'Schedule',
            title: 'Schedule',
            placeholder: 'Schedule',
            kinds: ['cron']
        },
        {
            id: 'triggers-cron-interval',
            path: 'attributes.interval',
            type: 'string',
            label: 'Interval',
            title: 'Interval',
            placeholder: 'Interval',
            kinds: ['cron']
        },
        {
            id: 'triggers-cron-event-body',
            path: 'attributes.event.body',
            type: 'string',
            label: 'Body',
            title: 'Body',
            placeholder: 'Body',
            kinds: ['cron']
        },

        // v3io stream specific
        {
            id: 'triggers-v3io-stream-num-container-workers',
            path: 'attributes.numContainerWorkers',
            type: 'number',
            label: 'Number of container workers',
            title: 'Number of container workers',
            placeholder: 'Number of container workers',
            kinds: ['v3ioStream']
        },
        {
            id: 'triggers-v3io-stream-seek-to',
            path: 'attributes.seekTo',
            type: 'dropdown',
            label: 'Seek to',
            options: [
                { value: '', label: 'Select seek to...' },
                { value: 'earliest', label: 'Earliest' },
                { value: 'latest', label: 'Latest' },
            ],
            kinds: ['v3ioStream']
        },
        {
            id: 'triggers-v3io-stream-read-batch-size',
            path: 'attributes.readBatchSize',
            type: 'number',
            label: 'Number of records to read from stream in a batch',
            title: 'Number of records to read from stream in a batch',
            placeholder: 'Number of records to read from stream in a batch',
            kinds: ['v3ioStream']
        },
        {
            id: 'triggers-v3io-stream-polling-interval-ms',
            path: 'attributes.pollingIntervalMs',
            type: 'number',
            label: 'Number of milliseconds to wait between record reads',
            title: 'Number of milliseconds to wait between record reads',
            placeholder: 'Number of milliseconds to wait between record reads',
            kinds: ['v3ioStream']
        }
    ];

    var triggersInput = createKeyValuePairsInput('triggers', {}, 'name', 'attributes',
        createCustomValueManipulator(triggerFields, triggerKinds));

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

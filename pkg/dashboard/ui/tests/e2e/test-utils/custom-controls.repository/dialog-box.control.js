function DialogBox(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var keyboard = e2e_imports.testUtil.keyboardUtils();
    var containerXPath;
    var content;
    var closeButton;
    var title;
    var articles;
    var notificationText;
    var cancelButton;
    var primaryButton;
    var removeButton;
    var errorText;
    var errorContainer;
    var textContentPreviewer;
    var imageContentPreviewer;

    index = index || 1;
    containerXPath = "(//div[contains(@class, 'ngdialog ')])[" + index + "]";

    // controls
    content = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']"));
    closeButton = control.get(by.xpath(containerXPath + "//div[contains(@class, 'close-button')]"));
    title = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']//div[contains(concat(' ', @class, ' '), ' title ')]"));
    articles = control.all(by.xpath(containerXPath + "//div[@class='ngdialog-content']//div[@class='field-label']"));
    notificationText = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']/div[contains(concat(' ', @class, ' '), ' notification-text ')]"));
    errorText = control.get(by.xpath(containerXPath + "//div[contains(@class,'error-text')]"));
    errorContainer = control.get(by.xpath(containerXPath + "//div[contains(@class,'error-container')]"));
    cancelButton = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']//div[contains(@class, 'igz-button-just-text')]"));
    primaryButton = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']//div[contains(@class,'igz-button-primary')][not(contains(@class,'hide'))]"));
    removeButton = control.get(by.xpath(containerXPath + "//div[@class='ngdialog-content']//div[contains(@class,'igz-button-remove')]"));
    textContentPreviewer = control.get(by.xpath(containerXPath + "//textarea[contains(@class, 'text-preview-container')]"));
    imageContentPreviewer = control.get(by.xpath(containerXPath + "//img[@class= 'image-preview']"));

    // methods
    /**
     * Check whether dialog box is visible
     * @param {boolean} isVisible
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    function isDialogBoxVisibility(isVisible) {
        return content.isPresence(isVisible)
            .then(function (presence) {
                return isVisible ? content.isVisibility(isVisible) : presence;
            });
    }

    /**
     * Close dialog box clicking on ESC keyboard button
     * @returns {!webdriver.promise.Promise}
     */
    function clickESCButton() {
        return keyboard.pressButtons(keyboard.getESCButton());
    }

    this.alert = function () {
        return {
            content: content,
            title: notificationText,
            okButton: primaryButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };

    this.confirm = function () {
        return {
            content: content,
            title: notificationText,
            cancelButton: cancelButton,
            okButton: primaryButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };

    this.delete = function () {
        return {
            content: content,
            title: notificationText,
            cancelButton: cancelButton,
            removeButton: removeButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };

    this.prompt = function () {
        return {
            content: content,
            title: title,
            articles: articles,
            errorText: errorText,
            errorContainer: errorContainer,
            closeButton: closeButton,
            cancelButton: cancelButton,
            okButton: primaryButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };

    this.textFileView = function () {
        return {
            content: content,
            title: title,
            contentPreviewer: textContentPreviewer,
            cancelButton: cancelButton,
            okButton: primaryButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };

    this.imageFileView = function () {
        return {
            content: content,
            title: title,
            contentPreviewer: imageContentPreviewer,
            closeButton: closeButton,
            isDialogBoxVisibility: isDialogBoxVisibility,
            exitOnESC: clickESCButton
        }
    };
}
module.exports = DialogBox;
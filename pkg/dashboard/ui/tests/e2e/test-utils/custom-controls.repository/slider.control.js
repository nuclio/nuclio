function Slider(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var mouseUtils = e2e_imports.testUtil.mouseUtils();
    //var containerXPath = "(" + parentElementXPath + "//rzslider)[" + index + "]";
    var _this = this;

    // controls
    this.slider = control.get(by.xpath("(" + parentElementXPath + "//span[@class='rz-bar'])[" + index + "]"));
    this.sliderSelection = control.get(by.xpath('(' + parentElementXPath + "//span[@class='rz-bar rz-selection'])[" + index + "]"));
    this.sliderPointer = function (index) {
        return control.get(by.xpath("(" + parentElementXPath + "//span[@role='slider'])[" + index + "]"));
    };
    this.sliderValue = function (index) {
        return control.get(by.xpath(parentElementXPath + "//span[contains(@class,'rz-bubble')][" + index + "]"));
    };

    // methods
    /**
     * Drag slider to set value
     * @param {number} value
     * returns {!webdriver.promise.Promise.<void>}
     */
    this.drag = function (value) {
        return mouseUtils.dragAndDrop(_this.sliderPointer(index), undefined, _this.slider, {x: value, y: 1});
    };

    /**
     * Moves point element on given steps count
     * @param {number} index
     * @param {number} steps
     */
    this.dragLinePoint = function (index, steps) {
        return getSegmentWidth()
            .then(function (stepWidth) {
                var dragWidth = Math.round(stepWidth * steps);
                return mouseUtils.dragAndDrop(_this.sliderPointer(index), undefined, _this.sliderPointer(index), {
                    x: dragWidth,
                    y: 0
                });
            });
    };

    this.getDragLineStepValue = function (index) {
        return _this.sliderPointer(index).getAttribute("aria-valuenow")
            .then(function (a) {
                return parseInt(a)
            })
    };

    /**
     * Returns a count of time filter drag line's segments
     * @returns {!webdriver.promise.Promise}
     */
    function getDragLineStepsCount() {
        return _this.getDragLineStepValue(1)
            .then(function (stepValue1) {
                return _this.getDragLineStepValue(2)
                    .then(function (stepValue2) {
                        return Math.abs(parseInt(stepValue2) - parseInt(stepValue1));
                    });
            });
    }

    /**
     * Returns a width of time filter drag line's segment
     * @returns {!webdriver.promise.Promise}
     */
    function getSegmentWidth() {
        return getDragLineStepsCount()
            .then(function (count) {
                return _this.sliderSelection.getSize().then(function (size) {
                    return size.width / count;
                });
            });
    }
}
module.exports = Slider;
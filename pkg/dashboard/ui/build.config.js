/**
 * This file/module contains all configuration for the build process.
 */
module.exports = {
    /**
     * Source folders
     */
    source_dir: 'src',

    /**
     * iguazio.dashboard-controls folders
     */
    shared_files: {
        dist: 'node_modules/iguazio.dashboard-controls/dist',
        src: 'node_modules/iguazio.dashboard-controls/src',
        less: 'node_modules/iguazio.dashboard-controls/src/**/*.less',
        js: [
            'node_modules/iguazio.dashboard-controls/src/iguazio.dashboard-controls.module.js',
            'node_modules/iguazio.dashboard-controls/src/igz_controls/**/*.js',
            '!node_modules/iguazio.dashboard-controls/src/igz_controls/**/*.spec.js',
            'node_modules/iguazio.dashboard-controls/src/nuclio/**/*.js',
            '!node_modules/iguazio.dashboard-controls/src/nuclio/**/*.spec.js'
        ],
        templates: 'node_modules/iguazio.dashboard-controls/src/**/*.html',
        fonts: 'node_modules/iguazio.dashboard-controls/src/igz_controls/fonts/**/*',
        images: 'node_modules/iguazio.dashboard-controls/src/igz_controls/images/*',
        templates_module_name: 'iguazio.dashboard-controls.templates'
    },

    /**
     * Destination folders
     */
    build_dir: 'dist',
    assets_dir: 'dist/assets',

    /**
     * Cache file
     */
    cache_file: '.babelCache',
    shared_cache_file: '.babelCacheShared',

    /**
     * App files and configs
     */
    app_files: {
        json: [
            'dashboard-config.json'
        ],
        js: [
            'src/app/app.module.js',
            'src/app/app.config.js',
            'src/app/app.route.js',
            'src/app/app.run.js',
            'src/app/app.controller.js',
            'src/app/components/**/*.js',
            '!src/app/components/**/*.spec.js',
            'src/app/shared/**/*.js',
            '!src/app/shared/**/*.spec.js',

            // js file with shared dashboard controls
            'node_modules/iguazio.dashboard-controls/dist/js/iguazio.dashboard-controls.js'
        ],
        html: 'src/index.html',
        less_files: [
            'node_modules/iguazio.dashboard-controls/dist/less/iguazio.dashboard-controls.less',

            'src/less/**/*.less',
            'src/app/components/**/*.less'
        ],
        fonts: 'node_modules/iguazio.dashboard-controls/dist/fonts',
        images: [
            'src/images/**/*',
            'node_modules/iguazio.dashboard-controls/dist/images/**/*'
        ],
        templates: 'src/app/components/**/*.tpl.html', // html files should be only in components folder
        templates_module_name: 'nuclio.app.templates'
    },

    /**
     * Configs used for testing
     */
    test_files: {
        unit: {
            vendor: [
                'node_modules/angular-mocks/angular-mocks.js'
            ],
            tests: [
                'src/**/*.spec.js'
            ],
            karma_config: 'tests/unit/karma.config.js'
        },
        e2e: {
            vendor: [
                'node_modules/angular-mocks/angular-mocks.js'
            ],
            mock_module: [
                'tests/e2e/mocks/angular-side/e2e.module.js',
                'tests/e2e/mocks/angular-side/mock-backend.service.js'
            ],
            protractor_config: 'tests/e2e/protractor.config.js',
            built_file_name: 'e2e.js',
            built_folder_name: 'dist/test',
            specs_location: 'tests/e2e/specs/',
            spec_path: {
                containers: 'tests/e2e/specs/containers/*.spec.js',
                login: 'tests/e2e/specs/login/*.spec.js',
                browse: 'tests/e2e/specs/containers/container/browse/**/*.spec.js',
                overview: 'tests/e2e/specs/containers/container/overview/**/*.spec.js',
                data_policy: 'tests/e2e/specs/containers/container/data-policy/**/*.spec.js',
                analytics: 'tests/e2e/specs/containers/container/analytics/**/*.spec.js',
                data_lifecycle: 'tests/e2e/specs/containers/container/data-lifecycle/**/*.spec.js',
                users: 'tests/e2e/specs/identity/users/**/*.spec.js',
                groups: 'tests/e2e/specs/identity/groups/**/*.spec.js',
                tenants: 'tests/e2e/specs/tenants/**/*.spec.js',
                networks: 'tests/e2e/specs/networks/**/*.spec.js',
                storage: 'tests/e2e/specs/storage/**/*.spec.js',
                events: 'tests/e2e/specs/events/**/*.spec.js',
                clusters: 'tests/e2e/specs/clusters/**/*.spec.js'
            }
        }
    },

    /**
     * Third-party libs (files order is important)
     */
    vendor_files: {
        js: [
            'node_modules/jquery/dist/jquery.js',
            'node_modules/angular/angular.js',
            'node_modules/angular-ui-bootstrap/dist/ui-bootstrap-tpls.js',
            'node_modules/@uirouter/angularjs/release/angular-ui-router.min.js',
            'node_modules/angular-ui-layout/src/ui-layout.js',
            'node_modules/jquery-ui/ui/widget.js',
            'node_modules/jquery-ui/ui/widgets/mouse.js',
            'node_modules/jquery-ui/ui/widgets/sortable.js',
            'node_modules/moment/moment.js',
            'node_modules/js-base64/base64.js',
            'node_modules/angular-animate/angular-animate.js',
            'node_modules/angular-cookies/angular-cookies.js',
            'node_modules/angular-money-directive/dist/angular-money-directive.js',
            'node_modules/yamljs/dist/yaml.js',
            'node_modules/angular-sanitize/angular-sanitize.js',
            'node_modules/angularjs-slider/dist/rzslider.js',
            'node_modules/bootstrap/js/dropdown.js',
            'node_modules/lodash/lodash.js',
            'node_modules/ng-dialog/js/ngDialog.js',
            'node_modules/malihu-custom-scrollbar-plugin/jquery.mCustomScrollbar.concat.min.js',
            'node_modules/ng-scrollbars/dist/scrollbars.min.js',
            'node_modules/monaco-editor/min/vs/loader.js',
            'node_modules/ng-file-upload/dist/ng-file-upload.js',
            'node_modules/ng-file-upload/dist/FileAPI.js',
            'node_modules/angular-download/angular-download.js'
        ],
        less: [
            'node_modules/bootstrap/less/bootstrap.less'
        ],
        css: [
            'node_modules/jquery-ui/themes/themes/theme.css',
            'node_modules/ng-dialog/css/ngDialog.css',
            'node_modules/malihu-custom-scrollbar-plugin/jquery.mCustomScrollbar.css',
            'node_modules/angular-ui-layout/src/ui-layout.css',
            'node_modules/angularjs-slider/dist/rzslider.css'
        ]
    },

    /**
     * Config for output files
     */
    output_files: {
        app: {
            js: 'app.js',
            css: 'app.css',
            js_manifest: 'app.manifest.json',
            css_manifest: 'app.manifest.json'
        },
        vendor: {
            js: 'vendor.js',
            css: 'vendor.css',
            js_manifest: 'vendor.manifest.json',
            css_manifest: 'vendor.manifest.json'
        }
    },

    /**
     * Config for output files
     */
    shared_output_files: {
        app: {
            js: 'iguazio.dashboard-controls.js',
            less: 'iguazio.dashboard-controls.less'
        }
    },

    /**
     * Config for resources
     */
    resources: {
        previewServer: './resources/previewServer',
        errHandler: './resources/gulpErrorHandler'
    }
};

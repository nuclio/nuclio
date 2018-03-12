function ImportsRepository() {
    this.mockInjector = require(e2e_root + '/mocks/protractor-side/mock-injector.js');

    /**
     * An object that contains methods for getting PageObject objects
     */
    this.pageObject = {
        mainHeader: function () {
            var MainHeader = require(e2e_root + '/specs/main-header/main-header.po.js');
            return new MainHeader();
        },
        mainSideMenu: function () {
            var MainHeader = require(e2e_root + '/specs/main-side-menu/main-side-menu.po.js');
            return new MainHeader();
        },
        newContainersNavigator: function () {
            var ContainersNavigator = require(e2e_root + '/specs/containers/container/navigator-tabs/navigator.po.js');
            return new ContainersNavigator();
        },
        newAnalyticsPage: function () {
            var AnalyticsPage = require(e2e_root + '/specs/containers/container/analytics/analytics.po.js');
            return new AnalyticsPage();
        },
        newDataAccessPolicyPage: function () {
            var DataAccessPolicyPage = require(e2e_root + '/specs/containers/container/data-policy/data-policy.po.js');
            return new DataAccessPolicyPage();
        },
        newDataLifecyclePage: function () {
            var DataLifecyclePage = require(e2e_root + '/specs/containers/container/data-lifecycle/data-lifecycle.po.js');
            return new DataLifecyclePage();
        },
        newDataLifecycleInfoPanePage: function () {
            var DataLifecycleInfoPanePage = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/info-pane.po.js');
            return new DataLifecycleInfoPanePage();
        },
        newDataLifecycleGeneralTab: function () {
            var DataLifecycleGeneralTab = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/general/general.po.js');
            return new DataLifecycleGeneralTab();
        },
        newDataLifecycleScheduleSection: function () {
            var DataLifecycleScheduleSection = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/policy-details/sections/schedule.po.js');
            return new DataLifecycleScheduleSection();
        },
        newDataLifecycleDestinationSection: function () {
            var DataLifecycleDestinationSection = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/policy-details/sections/destination.po.js');
            return new DataLifecycleDestinationSection();
        },
        newDataLifecycleNotificationsSection: function () {
            var DataLifecycleNotificationsSection = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/policy-details/sections/notifications.po.js');
            return new DataLifecycleNotificationsSection();
        },
        newDataLifecyclePolicySection: function () {
            var DataLifecyclePolicySection = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/policy-details/sections/policy.po.js');
            return new DataLifecyclePolicySection();
        },
        newDataLifecycleDataFiltersTab: function () {
            var DataLifecycleDataFiltersTab = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/data-filters/data-filters.po.js');
            return new DataLifecycleDataFiltersTab();
        },
        newInfoPanePage: function () {
            var InfoPanePage = require(e2e_root + '/specs/containers/container/data-policy/info-pane/info-pane.po.js');
            return new InfoPanePage();
        },
        newDataAccessPolicyGeneralTab: function () {
            var DataAccessPolicyGeneralTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/general/general.po.js');
            return new DataAccessPolicyGeneralTab();
        },
        newDataAccessPolicySourceTab: function () {
            var DataAccessPolicySourceTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/source/source.po.js');
            return new DataAccessPolicySourceTab();
        },
        newDataAccessPolicyUsersTab: function () {
            var DataAccessPolicyUsersTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/users/users.po.js');
            return new DataAccessPolicyUsersTab();
        },
        newDataAccessPolicyResourcesTab: function () {
            var DataAccessPolicyResourcesTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/resources/resources.po.js');
            return new DataAccessPolicyResourcesTab();
        },
        newDataAccessPolicyPermissionTab: function () {
            var DataAccessPolicyAccessTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/permission/permission.po.js');
            return new DataAccessPolicyAccessTab();
        },
        newDataAccessPolicySLATab: function () {
            var DataAccessPolicyAccessTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/sla/sla.po.js');
            return new DataAccessPolicyAccessTab();
        },
        newDataAccessPolicyFunctionsTab: function () {
            var DataAccessPolicyFunctionsTab = require(e2e_root + '/specs/containers/container/data-policy/info-pane/functions/functions.po.js');
            return new DataAccessPolicyFunctionsTab();
        },
        newLoginPage: function () {
            var LoginPage = require(e2e_root + '/specs/login/login.po.js');
            return new LoginPage();
        },
        newSignOutPage: function () {
          var SignOut = require(e2e_root + '/specs/signout/signout.po.js');
          return new SignOut();
        },
        newBrowsePage: function () {
            var Browse = require(e2e_root + '/specs/containers/container/browse/browse.po.js');
            return new Browse();
        },
        newBrowseInfoPane: function () {
            var BrowseInfoPane = require(e2e_root + '/specs/containers/container/browse/info-pane/info-pane.po.js');
            return new BrowseInfoPane();
        },
        newBrowseInfoPaneGeneralTab: function () {
            var BrowseInfoPaneGeneralTab = require(e2e_root + '/specs/containers/container/browse/info-pane/general/general.po.js');
            return new BrowseInfoPaneGeneralTab();
        },
        newBrowseInfoPaneAttributesTab: function () {
            var BrowseInfoPaneAttributesTab = require(e2e_root + '/specs/containers/container/browse/info-pane/attributes/attributes.po.js');
            return new BrowseInfoPaneAttributesTab();
        },
        newBrowseCsvEditor: function () {
            var CsvEditor = require(e2e_root + '/specs/containers/container/browse/csv-editor/csv-editor.po.js');
            return new CsvEditor();
        },
        newBrowseCsvEditorFieldsTab: function () {
            var CsvEditorFieldsTab = require(e2e_root + '/specs/containers/container/browse/csv-editor/fields-tab/csv-fields-tab.po.js');
            return new CsvEditorFieldsTab();
        },
        newBrowseCsvEditorNewFieldsDialogbox: function () {
            var NewFieldsDialogbox = require(e2e_root + '/specs/containers/container/browse/csv-editor/fields-tab/create-field/create-field.po.js');
            return new NewFieldsDialogbox();
        },
        newBrowseCsvEditorStatisticsTab: function () {
            var CsvEditorStatisticsTabControls = require(e2e_root + '/specs/containers/container/browse/csv-editor/statistics-tab/statistics-tab.po.js');
            return new CsvEditorStatisticsTabControls();
        },
        newPathDialogWindow: function () {
            var PathDialogWindow = require(e2e_root + '/specs/containers/container/browse/path-dialog/path-dialog.po.js');
            return new PathDialogWindow();
        },
        newBrowseInfoPanePermissionsTab: function () {
            var BrowseInfoPanePermissionsTab = require(e2e_root + '/specs/containers/container/browse/info-pane/permissions/permissions.po.js');
            return new BrowseInfoPanePermissionsTab();
        },
        newBrowseInfoPaneVersionsTab: function () {
            var BrowseInfoPaneVersionsTab = require(e2e_root + '/specs/containers/container/browse/info-pane/versions/versions.po.js');
            return new BrowseInfoPaneVersionsTab();
        },
        newFileViewPage: function () {
            var FileView = require(e2e_root + '/specs/containers/container/browse/file-view/file-view.po.js');
            return new FileView();
        },
        newTreeViewPage: function () {
            var TreeView = require(e2e_root + '/specs/containers/container/browse/tree-view/tree-view.po.js');
            return new TreeView();
        },
        newLowerPanePage: function () {
            var LowerPane = require(e2e_root + '/specs/containers/container/overview/lower-pane/lower-pane.po.js');
            return new LowerPane();
        },
        newOverviewPage: function () {
            var OverviewPage = require(e2e_root + '/specs/containers/container/overview/overview.po.js');
            return new OverviewPage();
        },
        newUpperPane: function () {
            var UpperPane = require(e2e_root + '/specs/containers/container/overview/upper-pane/upper-pane.po.js');
            return new UpperPane();
        },
        newInterfacesPane: function () {
            var InterfacesPane = require(e2e_root + '/specs/containers/container/overview/side-bar/interfaces-pane/interfaces-pane.po.js');
            return new InterfacesPane();
        },
        newOverviewSessionWindow: function () {
            var OverviewSessionWindow = require(e2e_root + '/specs/containers/container/overview/overview-sessions-window/overview-sessions-window.po.js');
            return new OverviewSessionWindow();
        },
        newTasksPane: function () {
            var TasksPane = require(e2e_root + '/specs/containers/container/overview/side-bar/tasks-pane/tasks-pane.po.js');
            return new TasksPane();
        },
        newContainersPage: function () {
            var Containers = require(e2e_root + '/specs/containers/containers.po.js');
            return new Containers();
        },
        newActivitiesPane: function () {
            var ActivitiesPane = require(e2e_root + '/specs/containers/container/overview/side-bar/activities-pane/activities-pane.po.js');
            return new ActivitiesPane();
        },
        eventsPage: function () {
            var EventPage = require(e2e_root + '/specs/events/events.po.js');
            return new EventPage();
        },
        eventsAlertsPage: function () {
            var EventAlertsPage = require(e2e_root + '/specs/events/alerts/alerts.po.js');
            return new EventAlertsPage();
        },
        eventsAlertsInfoPane: function () {
            var EventAlertsInfoPane = require(e2e_root + '/specs/events/alerts/info-pane/info-pane.po.js');
            return new EventAlertsInfoPane();
        },
        eventLogPage: function () {
            var EventLogPage = require(e2e_root + '/specs/events/event-log/event-log.po.js');
            return new EventLogPage();
        },
        eventLogInfoPane: function () {
            var EventLogInfoPane = require(e2e_root + '/specs/events/event-log/info-pane/info-pane.po.js');
            return new EventLogInfoPane();
        },
        eventLogInfoPaneInfoTab: function () {
            var EventLogInfoPaneInfoTab = require(e2e_root + '/specs/events/event-log/info-pane/info/info.po.js');
            return new EventLogInfoPaneInfoTab();
        },
        eventsEscalationPage: function () {
            var EventEscalationPage = require(e2e_root + '/specs/events/escalation/escalation.po.js');
            return new EventEscalationPage();
        },
        eventsEscalationInfoPane: function () {
            var EventEscalationInfoPane = require(e2e_root + '/specs/events/escalation/info-pane/info-pane.po.js');
            return new EventEscalationInfoPane();
        },
        eventsEscalationInfoPaneLayers: function () {
            var EventEscalationInfoPaneLayers = require(e2e_root + '/specs/events/escalation/info-pane/layers/layers.po.js');
            return new EventEscalationInfoPaneLayers();
        },
        eventsEscalationInfoPaneFilters: function () {
            var EventEscalationInfoPaneFilters = require(e2e_root + '/specs/events/escalation/info-pane/filters/filters.po.js');
            return new EventEscalationInfoPaneFilters();
        },
        tasksPage: function () {
            var TasksPage = require(e2e_root + '/specs/events/tasks/tasks.po.js');
            return new TasksPage();
        },
        tasksInfoPane: function () {
            var TasksInfoPane = require(e2e_root + '/specs/events/tasks/info-pane/info-pane.po.js');
            return new TasksInfoPane();
        },
        newUsersPage: function () {
            var UsersPage = require(e2e_root + '/specs/identity/users/users.po.js');
            return new UsersPage();
        },
        newTenantsPage: function () {
            var TenantsPage = require(e2e_root + '/specs/tenants/tenants.po.js');
            return new TenantsPage();
        },
        newGroupsPage: function () {
            var GroupsPage = require(e2e_root + '/specs/identity/groups/groups.po.js');
            return new GroupsPage();
        },
        newGroupsInfoPane: function () {
            var GroupsInfoPane = require(e2e_root + '/specs/identity/groups/info-pane/info-pane.po.js');
            return new GroupsInfoPane();
        },
        newUsersInfoPanePage: function () {
            var UsersInfoPanePage = require(e2e_root + '/specs/identity/users/info-pane/info-pane.po.js');
            return new UsersInfoPanePage();
        },
        newUsersInfoPaneDetails: function () {
            var UsersInfoPaneDetails = require(e2e_root + '/specs/identity/users/info-pane/details/details.po.js');
            return new UsersInfoPaneDetails();
        },
        newTenantsInfoPane: function () {
            var TenantsInfoPane = require(e2e_root + '/specs/tenants/info-pane/info-pane.po.js');
            return new TenantsInfoPane();
        },
        newUsersInfoPaneGroups: function () {
            var UsersInfoPaneGroups = require(e2e_root + '/specs/identity/users/info-pane/groups/groups.po.js');
            return new UsersInfoPaneGroups();
        },
        newUsersInfoPanePolicies: function () {
            var UsersInfoPanePolicies = require(e2e_root + '/specs/identity/users/info-pane/policies/policies.po.js');
            return new UsersInfoPanePolicies();
        },
        newNetworksPage: function () {
            var NetworksPage = require(e2e_root + '/specs/networks/networks.po.js');
            return new NetworksPage();
        },
        newNetworksInfoPane: function () {
            var NetworksInfoPane = require(e2e_root + '/specs/networks/info-pane/info-pane.po.js');
            return new NetworksInfoPane();
        },
        newStoragePoolsNavigator: function () {
            var StoragePoolsNavigator = require(e2e_root + '/specs/storage/navigator-tabs/navigator-tabs.po.js');
            return new StoragePoolsNavigator();
        },
        newStoragePoolsPage: function () {
            var StoragePoolsPage = require(e2e_root + '/specs/storage/storage-pools/storage-pools.po.js');
            return new StoragePoolsPage();
        },
        newStoragePoolsDevicesPage: function () {
            var StoragePoolsDevicesPage = require(e2e_root + '/specs/storage/devices/devices.po.js');
            return new StoragePoolsDevicesPage();
        },
        newStoragePoolsContainersPage: function () {
            var StoragePoolsContainersPage = require(e2e_root + '/specs/storage/containers/containers.po.js');
            return new StoragePoolsContainersPage();
        },
        newStoragePoolsInfoPane: function () {
            var StoragePoolsInfoPane = require(e2e_root + '/specs/storage/info-pane/info-pane.po.js');
            return new StoragePoolsInfoPane();
        },
        newStoragePoolsOverviewPage: function () {
            var StoragePoolsOverviewPage = require(e2e_root + '/specs/storage/overview/overview.po.js');
            return new StoragePoolsOverviewPage();
        },
        newStoragePoolsActivitiesPane: function () {
            var StoragePoolsActivitiesPane = require(e2e_root + '/specs/storage/overview/side-bar/activities-pane/activities-pane.po.js');
            return new StoragePoolsActivitiesPane();
        },
        newStoragePoolsContainersPane: function () {
            var StoragePoolsContainersPane = require(e2e_root + '/specs/storage/overview/side-bar/containers-pane/containers-pane.po.js');
            return new StoragePoolsContainersPane();
        },
        newStoragePoolsDevicesPane: function () {
            var StoragePoolsDevicesPane = require(e2e_root + '/specs/storage/overview/side-bar/devices-pane/devices-pane.po.js');
            return new StoragePoolsDevicesPane();
        },
        createStorageWizardPage: function () {
            var CreateStorageWizardPage = require(e2e_root + '/specs/storage/storage-pools/new-storage-wizard/new-storage-wizard.po.js');
            return new CreateStorageWizardPage();
        },
        clustersPage: function () {
            var ClustersPage = require(e2e_root + '/specs/clusters/clusters.po.js');
            return new ClustersPage();
        },
        clustersInfoPane: function () {
            var ClustersInfoPane = require(e2e_root + '/specs/clusters/info-pane/info-pane.po.js');
            return new ClustersInfoPane();
        },
        nodesPage: function () {
            var NodesPage = require(e2e_root + '/specs/clusters/nodes/nodes.po.js');
            return new NodesPage();
        },
        nodesInfoPane: function () {
            var NodesInfoPane = require(e2e_root + '/specs/clusters/nodes/info-pane/info-pane.po.js');
            return new NodesInfoPane();
        }
    };

    /**
     * An object that contains methods for getting mode data objects
     */
    this.modeData = {
        containersModeData: function () {
            var data = require(e2e_root + '/specs/containers/containers.mode-data.js');
            return data;
        },
        overviewModeData: function () {
            var data = require(e2e_root + '/specs/containers/container/overview/overview.mode-data.js');
            return data;
        },
        dataPolicyModeData: function () {
            var data = require(e2e_root + '/specs/containers/container/data-policy/data-policy.mode-data.js');
            return data;
        },
        browseModeData: function () {
            var data = require(e2e_root + '/specs/containers/container/browse/browse.mode-data.js');
            return data;
        },
        dataLifecycleModeData: function () {
            var data = require(e2e_root + '/specs/containers/container/data-lifecycle/data-lifecycle.mode-data.js');
            return data;
        },
        clustersModeData: function () {
            var data = require(e2e_root + '/specs/clusters/clusters.mode-data.js');
            return data;
        },
        storagePoolsModeData: function () {
            var data = require(e2e_root + '/specs/storage/storage-pools/storage-pools.mode-data.js');
            return data;
        },
        usersModeData: function () {
            var data = require(e2e_root + '/specs/identity/users/users.mode-data.js');
            return data;
        },
        groupsModeData: function () {
            var data = require(e2e_root + '/specs/identity/groups/groups.mode-data.js');
            return data;
        },
        eventsModeData: function () {
            var data = require(e2e_root + '/specs/events/events.mode-data.js');
            return data;
        },
        networksModeData: function () {
            var data = require(e2e_root + '/specs/networks/networks.mode-data.js');
            return data;
        }
    };

    /**
     * An object that contains methods for getting Utils objects
     */
    this.testUtil = {
        browserUtils: function () {
            var BrowserUtils = require(e2e_root + '/test-utils/browser.utils.js');
            return new BrowserUtils();
        },
        elementFinderUtils: function () {
            var ElementFinderUtils = require(e2e_root + '/test-utils/element-finder.utils.js');
            return new ElementFinderUtils();
        },
        customControlsRepository: function () {
            var CustomControls = require(e2e_root + '/test-utils/custom-controls.repository.js');
            return new CustomControls();
        },
        mouseUtils: function () {
            var MouseUtils = require(e2e_root + '/test-utils/mouse.utils.js');
            return new MouseUtils();
        },
        keyboardUtils: function () {
            var KeyboardUtils = require(e2e_root + '/test-utils/keyboard.utils.js');
            return new KeyboardUtils();
        },
        fileSystemUtils: function () {
            var FileSystemUtils = require(e2e_root + '/test-utils/file-system.utils.js');
            return new FileSystemUtils();
        },
        constants: function () {
            var Constants = require(e2e_root + '/test-utils/constants.repository.js');
            return new Constants();
        },
        htmlReporter: function () {
            var HtmlReporter = require(e2e_root + '/test-utils/html-reporter.utils.js');
            return new HtmlReporter();
        },
        mocks: function () {
            var Mocks = require(e2e_root + '/test-utils/mocks.repository.js');
            return new Mocks();
        },
        mockFormatter: function () {
            var MockFormatter = require(e2e_root + '/test-utils/mock-formatter.js');
            return new MockFormatter();
        },
        mockDataProvider: function () {
            var MockDataProvider = require(e2e_root + '/test-utils/mock-data-provider.js');
            return new MockDataProvider();
        },
        mockServer: function () {
            var MockServer = require(e2e_root + '/test-utils/mock-server.js');
            return new MockServer();
        },
        modeDataProvider: function () {
            return require(e2e_root + '/test-utils/mode-data-provider.js');
        }
    };

    /**
     * An object that contains methods for getting Controls objects
     */
    this.controlsObject = {
        loginControls: function () {
            var LoginControls = require(e2e_root + '/specs/login/login.controls.js');
            return new LoginControls();
        },
        signOutControls: function () {
            var SignOutControls = require(e2e_root + '/specs/signout/signout.controls.js');
            return new SignOutControls();
        },
        mainHeader: function () {
            var MainHeader = require(e2e_root + '/specs/main-header/main-header.controls.js');
            return new MainHeader();
        },
        mainSideMenu: function () {
            var MainHeader = require(e2e_root + '/specs/main-side-menu/main-side-menu.controls.js');
            return new MainHeader();
        },
        navigatorControls: function () {
            var NavigatorControls = require(e2e_root + '/specs/containers/container/navigator-tabs/navigator.controls.js');
            return new NavigatorControls();
        },
        analyticsControls: function () {
            var AnalyticsControls = require(e2e_root + '/specs/containers/container/analytics/analytics.controls.js');
            return new AnalyticsControls();
        },
        dataAccessPolicyControls: function () {
            var DataAccessPolicyControls = require(e2e_root + '/specs/containers/container/data-policy/data-policy.controls.js');
            return new DataAccessPolicyControls();
        },
        dataLifecycleControls: function () {
            var DataLifecycleControls = require(e2e_root + '/specs/containers/container/data-lifecycle/data-lifecycle.controls.js');
            return new DataLifecycleControls();
        },
        dataLifecycleInfoPaneControls: function () {
            var DataLifecycleInfoPaneControls = require(e2e_root + '/specs/containers/container/data-lifecycle/info-pane/info-pane.controls.js');
            return new DataLifecycleInfoPaneControls();
        },
        infoPaneControls: function () {
            var InfoPaneControls = require(e2e_root + '/specs/containers/container/data-policy/info-pane/info-pane.controls.js');
            return new InfoPaneControls();
        },
        fileViewControls: function () {
            var FileViewControls = require(e2e_root + '/specs/containers/container/browse/file-view/file-view.controls.js');
            return new FileViewControls();
        },
        browseControls: function () {
            var BrowseControls = require(e2e_root + '/specs/containers/container/browse/browse.controls.js');
            return new BrowseControls();
        },
        browseInfoPaneControls: function () {
            var BrowseInfoPaneControls = require(e2e_root + '/specs/containers/container/browse/info-pane/info-pane.controls.js');
            return new BrowseInfoPaneControls();
        },
        csvEditorControls: function () {
            var CsvEditorControls = require(e2e_root + '/specs/containers/container/browse/csv-editor/csv-editor.controls.js');
            return new CsvEditorControls();
        },
        csvEditorFieldsTabControls: function () {
            var CsvEditorFieldsTabControls = require(e2e_root + '/specs/containers/container/browse/csv-editor/fields-tab/csv-fields-tab.controls.js');
            return new CsvEditorFieldsTabControls();
        },
        csvEditorNewFieldsDialogboxControls: function () {
            var NewFieldsDialogboxControls = require(e2e_root + '/specs/containers/container/browse/csv-editor/fields-tab/create-field/create-field.controls.js');
            return new NewFieldsDialogboxControls();
        },
        csvEditorStatisticsTabControls: function () {
            var CsvEditorStatisticsTabControls = require(e2e_root + '/specs/containers/container/browse/csv-editor/statistics-tab/statistics-tab.controls.js');
            return new CsvEditorStatisticsTabControls();
        },
        pathDialogControls: function () {
            var PathDialogControls = require(e2e_root + '/specs/containers/container/browse/path-dialog/path-dialog.controls.js');
            return new PathDialogControls();
        },
        upperPaneControls: function () {
            var UpperPaneControls = require(e2e_root + '/specs/containers/container/overview/upper-pane/upper-pane.controls.js');
            return new UpperPaneControls();
        },
        lowerPaneControls: function () {
            var LowerPaneControls = require(e2e_root + '/specs/containers/container/overview/lower-pane/lower-pane.controls.js');
            return new LowerPaneControls();
        },
        interfacesPaneControls: function () {
            var InterfacesPaneControls = require(e2e_root + '/specs/containers/container/overview/side-bar/interfaces-pane/interfaces-pane.controls.js');
            return new InterfacesPaneControls();
        },
        overviewSessionWindowControls: function () {
            var OverviewSessionWindowControls = require(e2e_root + '/specs/containers/container/overview/overview-sessions-window/overview-sessions-window.controls.js');
            return new OverviewSessionWindowControls();
        },
        tasksPaneControls: function () {
            var TasksPaneControls = require(e2e_root + '/specs/containers/container/overview/side-bar/tasks-pane/tasks-pane.controls.js');
            return new TasksPaneControls();
        },
        containersControls: function () {
            var ContainersControls = require(e2e_root + '/specs/containers/containers.controls.js');
            return new ContainersControls();
        },
        activitiesPaneControls: function () {
            var ActivitiesPaneControls = require(e2e_root + '/specs/containers/container/overview/side-bar/activities-pane/activities-pane.controls.js');
            return new ActivitiesPaneControls();
        },
        eventsControls: function () {
            var EventPage = require(e2e_root + '/specs/events/events.controls.js');
            return new EventPage();
        },
        eventsAlertsControls: function () {
            var EventAlertsPage = require(e2e_root + '/specs/events/alerts/alerts.controls.js');
            return new EventAlertsPage();
        },
        eventsAlertsInfoPaneControls: function () {
            var AlertsInfoPaneControls = require(e2e_root + '/specs/events/alerts/info-pane/info-pane.controls.js');
            return new AlertsInfoPaneControls();
        },
        eventLogControls: function () {
            var EventLogControls = require(e2e_root + '/specs/events/event-log/event-log.controls.js');
            return new EventLogControls();
        },
        eventLogInfoPaneControls: function () {
            var EventLogInfoPaneControls = require(e2e_root + '/specs/events/event-log/info-pane/info-pane.controls.js');
            return new EventLogInfoPaneControls();
        },
        eventsEscalationControls: function () {
            var EventEscalationControls = require(e2e_root + '/specs/events/escalation/escalation.controls.js');
            return new EventEscalationControls();
        },
        eventsEscalationInfoPaneControls: function () {
            var EventsEscalationInfoPaneControls = require(e2e_root + '/specs/events/escalation/info-pane/info-pane.controls.js');
            return new EventsEscalationInfoPaneControls();
        },
        tasksControls: function () {
            var TasksControls = require(e2e_root + '/specs/events/tasks/tasks.controls.js');
            return new TasksControls();
        },
        tasksInfoPaneControls: function () {
            var TasksInfoPaneControls = require(e2e_root + '/specs/events/tasks/info-pane/info-pane.controls.js');
            return new TasksInfoPaneControls();
        },
        usersControls: function () {
            var UsersControls = require(e2e_root + '/specs/identity/users/users.controls.js');
            return new UsersControls();
        },
        tenantsControls: function () {
            var TenantsControls = require(e2e_root + '/specs/tenants/tenants.controls.js');
            return new TenantsControls();
        },

        groupsControls: function () {
            var GroupsControls = require(e2e_root + '/specs/identity/groups/groups.controls.js');
            return new GroupsControls();
        },
        groupsInfoPaneControls: function () {
            var GroupsInfoPaneControls = require(e2e_root + '/specs/identity/groups/info-pane/info-pane.controls.js');
            return new GroupsInfoPaneControls();
        },
        usersInfoPaneControls: function () {
            var UsersInfoPaneControls = require(e2e_root + '/specs/identity/users/info-pane/info-pane.controls.js');
            return new UsersInfoPaneControls();
        },
        networksControls: function () {
            var NetworksControls = require(e2e_root + '/specs/networks/networks.controls.js');
            return new NetworksControls();
        },
        networksInfoPaneControls: function () {
            var NetworksInfoPaneControls = require(e2e_root + '/specs/networks/info-pane/info-pane.controls.js');
            return new NetworksInfoPaneControls();
        },
        storagePoolsControls: function () {
            var StoragePoolsControls = require(e2e_root + '/specs/storage/storage-pools/storage-pools.controls.js');
            return new StoragePoolsControls();
        },
        storagePoolsDevicesControls: function () {
            var StoragePoolsDevicesControls = require(e2e_root + '/specs/storage/devices/devices.controls.js');
            return new StoragePoolsDevicesControls();
        },
        storagePoolsContainersControls: function () {
            var StoragePoolsContainersControls = require(e2e_root + '/specs/storage/containers/containers.controls.js');
            return new StoragePoolsContainersControls();
        },
        storagePoolsInfoPaneControls: function () {
            var StoragePoolsInfoPaneControls = require(e2e_root + '/specs/storage/info-pane/info-pane.controls.js');
            return new StoragePoolsInfoPaneControls();
        },
        storagePoolsOverviewControls: function () {
            var StoragePoolsOverviewControls = require(e2e_root + '/specs/storage/overview/overview.controls.js');
            return new StoragePoolsOverviewControls();
        },
        storagePoolsActivitiesPaneControls: function () {
            var StoragePoolsActivitiesPaneControls = require(e2e_root + '/specs/storage/overview/side-bar/activities-pane/activities-pane.controls.js');
            return new StoragePoolsActivitiesPaneControls();
        },
        storagePoolsContainersPaneControls: function () {
            var StoragePoolsContainersPaneControls = require(e2e_root + '/specs/storage/overview/side-bar/containers-pane/containers-pane.controls.js');
            return new StoragePoolsContainersPaneControls();
        },
        storagePoolsDevicesPaneControls: function () {
            var StoragePoolsDevicesPaneControls = require(e2e_root + '/specs/storage/overview/side-bar/devices-pane/devices-pane.controls.js');
            return new StoragePoolsDevicesPaneControls();
        },
        createStorageWizardControls: function () {
            var CreateStorageWizardControls = require(e2e_root + '/specs/storage/storage-pools/new-storage-wizard/new-storage-wizard.controls.js');
            return new CreateStorageWizardControls();
        },
        clustersControls: function () {
            var ClustersAlertsPage = require(e2e_root + '/specs/clusters/clusters.controls.js');
            return new ClustersAlertsPage();
        },
        clustersInfoPaneControls: function () {
            var ClustersInfoPaneControls = require(e2e_root + '/specs/clusters/info-pane/info-pane.controls.js');
            return new ClustersInfoPaneControls();
        },
        nodesControls: function () {
            var NodesControls = require(e2e_root + '/specs/clusters/nodes/nodes.controls.js');
            return new NodesControls();
        },
        nodesInfoPaneControls: function () {
            var NodesInfoPaneControls = require(e2e_root + '/specs/clusters/nodes/info-pane/info-pane.controls.js');
            return new NodesInfoPaneControls();
        },
        tenantsInfoPaneControls: function () {
            var TenantsInfoPaneControls = require(e2e_root + '/specs/tenants/info-pane/info-pane.controls.js');
            return new TenantsInfoPaneControls();
        }
    };
}
module.exports = ImportsRepository;
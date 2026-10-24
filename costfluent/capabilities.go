package costfluent

// Capability is a right an API token carries, spelled as the Public API sends it in GET /v1/me and
// as the console's token dialog names it, such as "viewCosts". TestCapabilitiesMatchTheContract
// holds this list to the API's ApiTokenCapability values. A capability the API adds later decodes
// as its string, so a client should tolerate one it does not list.
type Capability string

const (
	CapabilityViewOrganization              Capability = "viewOrganization"
	CapabilityViewSubscription              Capability = "viewSubscription"
	CapabilityProvisionIdentities           Capability = "provisionIdentities"
	CapabilityCreateWorkspace               Capability = "createWorkspace"
	CapabilityViewWorkspace                 Capability = "viewWorkspace"
	CapabilityUpdateWorkspace               Capability = "updateWorkspace"
	CapabilityDeleteWorkspace               Capability = "deleteWorkspace"
	CapabilityManageWorkspaceAccess         Capability = "manageWorkspaceAccess"
	CapabilityInviteUser                    Capability = "inviteUser"
	CapabilityViewMembers                   Capability = "viewMembers"
	CapabilityCreateTeam                    Capability = "createTeam"
	CapabilityManageTeam                    Capability = "manageTeam"
	CapabilityDeleteTeam                    Capability = "deleteTeam"
	CapabilityViewProviders                 Capability = "viewProviders"
	CapabilityManageProviders               Capability = "manageProviders"
	CapabilitySyncProviders                 Capability = "syncProviders"
	CapabilityReportKubernetesUsage         Capability = "reportKubernetesUsage"
	CapabilityViewCosts                     Capability = "viewCosts"
	CapabilityViewBudgets                   Capability = "viewBudgets"
	CapabilityCreateBudget                  Capability = "createBudget"
	CapabilityUpdateBudget                  Capability = "updateBudget"
	CapabilityDeleteBudget                  Capability = "deleteBudget"
	CapabilityViewCostReports               Capability = "viewCostReports"
	CapabilityCreateCostReport              Capability = "createCostReport"
	CapabilityUpdateCostReport              Capability = "updateCostReport"
	CapabilityDeleteCostReport              Capability = "deleteCostReport"
	CapabilityViewDashboards                Capability = "viewDashboards"
	CapabilityCreateDashboard               Capability = "createDashboard"
	CapabilityUpdateDashboard               Capability = "updateDashboard"
	CapabilityDeleteDashboard               Capability = "deleteDashboard"
	CapabilityViewAnomalies                 Capability = "viewAnomalies"
	CapabilityAcknowledgeAnomaly            Capability = "acknowledgeAnomaly"
	CapabilityAddWorkspaceProvider          Capability = "addWorkspaceProvider"
	CapabilityRemoveWorkspaceProvider       Capability = "removeWorkspaceProvider"
	CapabilityCreateSavedFilter             Capability = "createSavedFilter"
	CapabilityUpdateSavedFilter             Capability = "updateSavedFilter"
	CapabilityDeleteSavedFilter             Capability = "deleteSavedFilter"
	CapabilityCreateFolder                  Capability = "createFolder"
	CapabilityUpdateFolder                  Capability = "updateFolder"
	CapabilityDeleteFolder                  Capability = "deleteFolder"
	CapabilityCreateApp                     Capability = "createApp"
	CapabilityUpdateApp                     Capability = "updateApp"
	CapabilityDeleteApp                     Capability = "deleteApp"
	CapabilityManageAlerts                  Capability = "manageAlerts"
	CapabilityViewCostReportSubscriptions   Capability = "viewCostReportSubscriptions"
	CapabilityManageCostReportSubscriptions Capability = "manageCostReportSubscriptions"
	CapabilityViewTags                      Capability = "viewTags"
	CapabilityManageTags                    Capability = "manageTags"
	CapabilityManageTagKeys                 Capability = "manageTagKeys"
	CapabilityViewAuditLog                  Capability = "viewAuditLog"
	CapabilityViewCostAllocation            Capability = "viewCostAllocation"
	CapabilityManageCostAllocation          Capability = "manageCostAllocation"
	CapabilityManageBusinessMetrics         Capability = "manageBusinessMetrics"
)

// Capabilities lists every capability, in the order the API declares them.
func Capabilities() []Capability {
	return []Capability{
		CapabilityViewOrganization,
		CapabilityViewSubscription,
		CapabilityProvisionIdentities,
		CapabilityCreateWorkspace,
		CapabilityViewWorkspace,
		CapabilityUpdateWorkspace,
		CapabilityDeleteWorkspace,
		CapabilityManageWorkspaceAccess,
		CapabilityInviteUser,
		CapabilityViewMembers,
		CapabilityCreateTeam,
		CapabilityManageTeam,
		CapabilityDeleteTeam,
		CapabilityViewProviders,
		CapabilityManageProviders,
		CapabilitySyncProviders,
		CapabilityReportKubernetesUsage,
		CapabilityViewCosts,
		CapabilityViewBudgets,
		CapabilityCreateBudget,
		CapabilityUpdateBudget,
		CapabilityDeleteBudget,
		CapabilityViewCostReports,
		CapabilityCreateCostReport,
		CapabilityUpdateCostReport,
		CapabilityDeleteCostReport,
		CapabilityViewDashboards,
		CapabilityCreateDashboard,
		CapabilityUpdateDashboard,
		CapabilityDeleteDashboard,
		CapabilityViewAnomalies,
		CapabilityAcknowledgeAnomaly,
		CapabilityAddWorkspaceProvider,
		CapabilityRemoveWorkspaceProvider,
		CapabilityCreateSavedFilter,
		CapabilityUpdateSavedFilter,
		CapabilityDeleteSavedFilter,
		CapabilityCreateFolder,
		CapabilityUpdateFolder,
		CapabilityDeleteFolder,
		CapabilityCreateApp,
		CapabilityUpdateApp,
		CapabilityDeleteApp,
		CapabilityManageAlerts,
		CapabilityViewCostReportSubscriptions,
		CapabilityManageCostReportSubscriptions,
		CapabilityViewTags,
		CapabilityManageTags,
		CapabilityManageTagKeys,
		CapabilityViewAuditLog,
		CapabilityViewCostAllocation,
		CapabilityManageCostAllocation,
		CapabilityManageBusinessMetrics,
	}
}

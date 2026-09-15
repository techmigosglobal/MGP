package view

type PassRow struct {
	ID         string
	PassNo     string
	PassDate   string
	Consignee  string
	PassType   string
	Status     string
	CanOpen    bool
	CanSubmit  bool
	CanApprove bool
	CanPassOut bool
	CanReturn  bool
}

type PassDetail struct {
	ID                    string
	PassNo                string
	PassDate              string
	PassType              string
	Status                string
	ExpectedReturnDate    string
	ExpectedReturnDateISO string
	ActualReturnDate      string
	Directorate           string
	Project               string
	Consignee             string
	ConsigneeAddress      string
	ReferenceNo           string
	Packages              int
	Purpose               string
	Authority             string
	InventoryNo           string
	InventoryHolder       string
	VehicleNo             string
	LoadedInPresenceOf    string
	CarrierName           string
	CarrierDesignation    string
	Remarks               string
	CopyType              string
	RejectionReason       string
	SecurityControlNo     string
	CreatedBy             string
	CreatedByName         string
	ApprovedBy            string
	ApprovedByName        string
	SecurityOfficer       string
	SecurityOfficerName   string
	ReturnedBy            string
	ReturnedByName        string
	Items                 []PassItem
}

type PassItem struct {
	Code        string
	Name        string
	Category    string
	SerialNo    string
	BatchNo     string
	FullPart    string
	Unit        string
	Quantity    string
	Description string
}

type NewPassData struct {
	PageData
	RevisionOf         string
	PassType           string
	PassDate           string
	ExpectedReturnDate string
	Directorate        string
	Project            string
	ConsigneeName      string
	ConsigneeAddress   string
	ReferenceNo        string
	Packages           string
	Purpose            string
	Authority          string
	InventoryNo        string
	InventoryHolder    string
	VehicleNo          string
	LoadedInPresenceOf string
	CarrierName        string
	CarrierDesignation string
	Remarks            string
	CopyType           string
	Items              []PassFormItem
}

type PassFormItem struct {
	Code        string
	Name        string
	Category    string
	SerialNo    string
	BatchNo     string
	FullPart    string
	Unit        string
	Quantity    string
	Description string
}

type DetailData struct {
	PageData
	Pass PassDetail
}

type UserRow struct {
	ID            string
	Username      string
	Name          string
	Role          string
	Status        string
	Rank          string
	Phone         string
	SignaturePath string
}

type UsersData struct {
	PageData
	Users []UserRow
}

type MasterData struct {
	PageData
	Inventory  []MasterInventoryRow
	Consignees []MasterConsigneeRow
}

type MasterInventoryRow struct{ ID, Code, Name, Category, SerialNo, BatchNo, Unit, Quantity, Holder, Description, Status string }
type MasterConsigneeRow struct{ ID, Name, Address, Contact, Status string }

type AuditData struct {
	PageData
	Events []AuditRow
}
type AuditRow struct{ CreatedAt, EntityType, Action, ActorRole, Reason, Metadata string }

type SettingsData struct {
	PageData
	ApplicationName, OrganizationName, OrganizationAddress, DefaultDirectorate, DefaultProject string
	DefaultCopy, AllowManualPassNo, SessionMinutes, Logo                                       string
}

type PageData struct {
	Title           string
	Active          string
	UserName        string
	Role            string
	CSRFToken       string
	Notice          string
	Error           string
	Search          string
	StatusFilter    string
	TypeFilter      string
	Passes          []PassRow
	Visible         int
	Pending         int
	PassedOut       int
	Overdue         int
	Notifications   int
	TestCredentials bool
}

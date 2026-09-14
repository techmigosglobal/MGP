package view

type PassRow struct {
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
	PassNo             string
	PassDate           string
	PassType           string
	Status             string
	ExpectedReturnDate string
	ActualReturnDate   string
	Directorate        string
	Project            string
	Consignee          string
	Packages           int
	Purpose            string
	Authority          string
	RejectionReason    string
	CreatedBy          string
	ApprovedBy         string
	SecurityOfficer    string
	ReturnedBy         string
	Items              []PassItem
}

type PassItem struct {
	Code     string
	Name     string
	Unit     string
	Quantity string
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
	Packages           string
	Purpose            string
	Authority          string
	ItemCode           string
	ItemName           string
	ItemUnit           string
	ItemQuantity       string
}

type DetailData struct {
	PageData
	Pass PassDetail
}

type UserRow struct {
	ID     string
	Email  string
	Name   string
	Role   string
	Status string
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

type MasterInventoryRow struct{ ID, Code, Name, Category, Unit, Quantity, Holder, Status string }
type MasterConsigneeRow struct{ ID, Name, Address, Contact, Status string }

type AuditData struct {
	PageData
	Events []AuditRow
}
type AuditRow struct{ CreatedAt, EntityType, Action, ActorRole, Reason, Metadata string }

type SettingsData struct {
	PageData
	OrganizationName, OrganizationAddress, DefaultDirectorate, DefaultProject string
}

type PageData struct {
	Title     string
	Active    string
	UserName  string
	Role      string
	CSRFToken string
	Notice    string
	Error     string
	Passes    []PassRow
	Visible   int
	Pending   int
	PassedOut int
	Overdue   int
}

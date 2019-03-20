package in

type EventSubForm struct {
	Id string

	Caption string

	Rank int32

	Fields *ArrayOfEventField
}

type ArrayOfEventField struct {
	EventField []*EventField
}

type EventField struct {
	Id string

	Name string

	FieldType string

	IsMandatory bool

	Rank int32

	Tag string

	Items *ArrayOfEventFieldItem

	CustomFieldOrder int32
}

type ArrayOfEventFieldItem struct {
	EventFieldItem []*EventFieldItem
}

type EventFieldItem struct {
	Id string

	Text string

	Rank int32
}

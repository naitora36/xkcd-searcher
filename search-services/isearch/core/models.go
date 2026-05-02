package core

type Comic struct {
	ID  int
	URL string
}
type SearchRequest struct {
	Phrase string
	Limit  int
}
type SearchReply struct {
	Comics []Comic
}
type DBComic struct {
	ID    int
	URL   string
	Words []string
}
type DBLightComic struct {
	ID  int
	URL string
}
type SearchIndex struct {
	Index map[string][]int
}

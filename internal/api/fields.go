package api

// fields.go — списки полей fields для каждой операции v1 (SPEC §4.2, Приложение А).
// Порядок полей стабильный и является единым источником истины: вывод команд
// детерминирован (важно для golden-тестов, SPEC §5.2).

const (
	// FieldsAuthLogin — GET /users/me при yt auth login (SPEC §3.3).
	FieldsAuthLogin = "id,login,fullName,email"

	// FieldsAuthStatus — GET /users/me при yt auth status (SPEC §3.3).
	FieldsAuthStatus = "id,login,fullName,email,guest"

	// FieldsUserWhoami — GET /users/me при yt user whoami (SPEC §3.8).
	FieldsUserWhoami = "id,login,fullName,email,guest,avatarUrl"

	// FieldsIssueList — GET /issues при yt issue list и yt search (SPEC §3.4, §3.5).
	FieldsIssueList = "id,idReadable,summary,created,updated,resolved,project(id,shortName),reporter(id,login,fullName),customFields(id,name,value($type,name))"

	// FieldsIssueView — GET /issues/{id} при yt issue view (SPEC §3.4).
	FieldsIssueView = "id,idReadable,summary,description,created,updated,resolved,project(id,shortName,name),reporter(id,login,fullName,email),updater(id,login,fullName),customFields(id,name,value($type,id,name,login,fullName,minutes,minutesPerDay,presentation)),tags(id,name),commentsCount"

	// FieldsIssueComments — GET /issues/{id}/comments при yt issue comment list (SPEC §3.4).
	FieldsIssueComments = "$type,id,text,created,author(id,login,fullName)"

	// FieldsIssueCommentCreate — POST /issues/{id}/comments при yt issue comment create (SPEC §3.4).
	FieldsIssueCommentCreate = "$type,id,text,created,author(id,login)"

	// FieldsIssueCreate — POST /issues при yt issue create (SPEC §3.4).
	FieldsIssueCreate = "id,idReadable,summary,project(id,shortName)"

	// FieldsIssueEdit — POST /issues/{id} при yt issue edit (SPEC §3.4).
	FieldsIssueEdit = "id,idReadable,summary,description"

	// FieldsCommandIssues — POST /commands при yt issue close и yt command (SPEC §3.4, §3.6).
	FieldsCommandIssues = "issues(id,idReadable,summary,resolved,project(id,shortName))"

	// FieldsSearchSuggest — POST /search/assist при yt search suggest (SPEC §3.5).
	FieldsSearchSuggest = "query,suggestions(option,description,prefix,suffix,group)"

	// FieldsCommandAssist — POST /commands/assist при yt command assist (SPEC §3.6).
	FieldsCommandAssist = "query,suggestions(option,description,prefix,suffix,group)"

	// FieldsProjectList — GET /admin/projects при yt project list (SPEC §3.7).
	FieldsProjectList = "id,name,shortName,archived,leader(id,login,fullName)"

	// FieldsProjectResolve — GET /admin/projects при резолвинге проекта в yt issue create (SPEC §3.4).
	FieldsProjectResolve = "id,shortName,name"

	// FieldsTagList — GET /tags при yt tag list (SPEC §3.9).
	FieldsTagList = "id,name,untagOnResolve"
)

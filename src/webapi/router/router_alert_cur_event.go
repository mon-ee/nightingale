package router

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"

	"github.com/didi/nightingale/v5/src/models"
)

func parseAggrRules(c *gin.Context) []*models.AggrRule {
	aggrRules := strings.Split(ginx.QueryStr(c, "rules", ""), "::") // e.g. field:group_name::field:severity::tagkey:ident

	if len(aggrRules) == 0 {
		ginx.Bomb(http.StatusBadRequest, "rules empty")
	}

	rules := make([]*models.AggrRule, len(aggrRules))
	for i := 0; i < len(aggrRules); i++ {
		pair := strings.Split(aggrRules[i], ":")
		if len(pair) != 2 {
			ginx.Bomb(http.StatusBadRequest, "rules invalid")
		}

		if !(pair[0] == "field" || pair[0] == "tagkey") {
			ginx.Bomb(http.StatusBadRequest, "rules invalid")
		}

		rules[i] = &models.AggrRule{
			Type:  pair[0],
			Value: pair[1],
		}
	}

	return rules
}

func alertCurEventsCard(c *gin.Context) {
	stime, etime := getTimeRange(c)
	severity := ginx.QueryInt(c, "severity", -1)
	query := ginx.QueryStr(c, "query", "")
	busiGroupId := ginx.QueryInt64(c, "bgid", 0)
	clusters := queryClusters(c)
	rules := parseAggrRules(c)

	// 最多获取50000个，获取太多也没啥意义
	list, err := models.AlertCurEventGets(busiGroupId, stime, etime, severity, clusters, query, 50000, 0)
	ginx.Dangerous(err)

	cardmap := make(map[string]*AlertCard)
	for _, event := range list {
		title := event.GenCardTitle(rules)
		if _, has := cardmap[title]; has {
			cardmap[title].Total++
			cardmap[title].EventIds = append(cardmap[title].EventIds, event.Id)
			if event.Severity < cardmap[title].Severity {
				cardmap[title].Severity = event.Severity
			}
		} else {
			cardmap[title] = &AlertCard{
				Total:    1,
				EventIds: []int64{event.Id},
				Title:    title,
				Severity: event.Severity,
			}
		}
	}

	titles := make([]string, 0, len(cardmap))
	for title := range cardmap {
		titles = append(titles, title)
	}

	sort.Strings(titles)

	cards := make([]*AlertCard, len(titles))
	for i := 0; i < len(titles); i++ {
		cards[i] = cardmap[titles[i]]
	}

	ginx.NewRender(c).Data(cards, nil)
}

type AlertCard struct {
	Title    string  `json:"title"`
	Total    int     `json:"total"`
	EventIds []int64 `json:"event_ids"`
	Severity int     `json:"severity"`
}

func alertCurEventsCardDetails(c *gin.Context) {
	var f idsForm
	ginx.BindJSON(c, &f)

	list, err := models.AlertCurEventGetByIds(f.Ids)
	if err == nil {
		cache := make(map[int64]*models.UserGroup)
		for i := 0; i < len(list); i++ {
			list[i].FillNotifyGroups(cache)
		}
	}

	ginx.NewRender(c).Data(list, err)
}

// 列表方式，拉取活跃告警
func alertCurEventsList(c *gin.Context) {
	stime, etime := getTimeRange(c)
	severity := ginx.QueryInt(c, "severity", -1)
	query := ginx.QueryStr(c, "query", "")
	limit := ginx.QueryInt(c, "limit", 20)
	busiGroupId := ginx.QueryInt64(c, "bgid", 0)
	clusters := queryClusters(c)

	total, err := models.AlertCurEventTotal(busiGroupId, stime, etime, severity, clusters, query)
	ginx.Dangerous(err)

	list, err := models.AlertCurEventGets(busiGroupId, stime, etime, severity, clusters, query, limit, ginx.Offset(c, limit))
	ginx.Dangerous(err)

	cache := make(map[int64]*models.UserGroup)
	for i := 0; i < len(list); i++ {
		list[i].FillNotifyGroups(cache)
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  list,
		"total": total,
	}, nil)
}

func alertCurEventGets(c *gin.Context) {
	stime, etime := getTimeRange(c)

	severity := ginx.QueryInt(c, "severity", -1)
	query := ginx.QueryStr(c, "query", "")
	limit := ginx.QueryInt(c, "limit", 20)
	busiGroupId := ginx.UrlParamInt64(c, "id")
	clusters := queryClusters(c)

	total, err := models.AlertCurEventTotal(busiGroupId, stime, etime, severity, clusters, query)
	ginx.Dangerous(err)

	list, err := models.AlertCurEventGets(busiGroupId, stime, etime, severity, clusters, query, limit, ginx.Offset(c, limit))
	ginx.Dangerous(err)

	cache := make(map[int64]*models.UserGroup)
	for i := 0; i < len(list); i++ {
		list[i].FillNotifyGroups(cache)
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  list,
		"total": total,
	}, nil)
}

func alertCurEventDel(c *gin.Context) {
	var f idsForm
	ginx.BindJSON(c, &f)
	f.Verify()

	set := make(map[int64]struct{})

	for i := 0; i < len(f.Ids); i++ {
		event, err := models.AlertCurEventGetById(f.Ids[i])
		ginx.Dangerous(err)

		if _, has := set[event.GroupId]; !has {
			bgrwCheck(c, event.GroupId)
			set[event.GroupId] = struct{}{}
		}
	}

	ginx.NewRender(c).Message(models.AlertCurEventDel(f.Ids))
}

func alertCurEventGet(c *gin.Context) {
	eid := ginx.UrlParamInt64(c, "eid")
	event, err := models.AlertCurEventGetById(eid)
	ginx.Dangerous(err)

	if event == nil {
		ginx.Bomb(404, "No such active event")
	}

	ginx.NewRender(c).Data(event, nil)
}

package language

import (
	"fmt"
	"github.com/Aptomi/aptomi/pkg/slinga/language/expression"
	. "github.com/Aptomi/aptomi/pkg/slinga/log"
	log "github.com/Sirupsen/logrus"
)

// LabelsFilter is a labels filter
type LabelsFilter []string

// ServiceFilter is a service filter
type ServiceFilter struct {
	Cluster *Criteria
	Labels  *Criteria
	User    *Criteria
}

// Action is an action
type Action struct {
	Type    string
	Content string
}

// Rule is a global rule
type Rule struct {
	*SlingaObject

	FilterServices *ServiceFilter
	Actions        []*Action
}

// DescribeConditions returns full description of the rule - conditions and actions description
func (rule *Rule) DescribeConditions() map[string][]string {
	descr := make(map[string][]string)

	if rule.FilterServices != nil {
		userFilter := rule.FilterServices.User
		if userFilter != nil {
			if len(userFilter.RequireAny) > 0 {
				descr["User with labels matching"] = userFilter.RequireAny
			}
			if len(userFilter.RequireNone) > 0 {
				descr["User without labels matching"] = userFilter.RequireNone
			}
		}
		clusterFilter := rule.FilterServices.Cluster
		if clusterFilter != nil {
			if len(clusterFilter.RequireAny) > 0 {
				descr["Cluster with labels matching"] = clusterFilter.RequireAny
			}
			if len(clusterFilter.RequireNone) > 0 {
				descr["Cluster without labels matching"] = clusterFilter.RequireNone
			}
		}
	}

	return descr
}

// DescribeActions describes all actions
func (rule *Rule) DescribeActions() []string {
	descr := make([]string, 0)

	for _, action := range rule.Actions {
		if action.Type == "dependency" && action.Content == "forbid" {
			descr = append(descr, "Forbid using services")
		} else if action.Type == "ingress" && action.Content == "block" {
			descr = append(descr, "Block external access to services")
		} else {
			descr = append(descr, fmt.Sprintf("type: %s, content: %s", action.Type, action.Content))
		}
	}

	return descr
}

// MatchUser returns if a rue matches a user
func (rule *Rule) MatchUser(user *User) bool {
	return rule.FilterServices != nil && rule.FilterServices.Match(NewLabelSetEmpty(), user, nil, nil)
}

// GlobalRules is a list of global rules
type GlobalRules struct {
	// action type -> []*Rule
	Rules map[string][]*Rule
}

// AllowsIngressAccess returns true if a rule allows ingress access
func (globalRules *GlobalRules) AllowsIngressAccess(labels LabelSet, users []*User, cluster *Cluster) bool {
	if rules, ok := globalRules.Rules["ingress"]; ok {
		for _, rule := range rules {
			// for all users of the service
			for _, user := range users {
				// TODO: this is pretty shitty that it's not a part of engine_node
				//       you can't log into "rule log" (new replacement of tracing)
				//       you can't use engine cache for expressions/template
				if rule.FilterServices.Match(labels, user, cluster, nil) {
					for _, action := range rule.Actions {
						if action.Type == "ingress" && action.Content == "block" {
							return false
						}
					}
				}
			}
		}
	}

	return true
}

// Match returns if a given parameters match a service filter
func (filter *ServiceFilter) Match(labels LabelSet, user *User, cluster *Cluster, cache expression.ExpressionCache) bool {
	// check if service filters for another service labels
	if filter.Labels != nil && !filter.Labels.allows(expression.NewExpressionParams(labels.Labels, nil), cache) {
		return false
	}

	// check if service filters for another user labels
	if filter.User != nil && !filter.User.allows(expression.NewExpressionParams(user.GetLabelSet().Labels, nil), cache) {
		return false
	}

	if filter.Cluster != nil && cluster != nil && !filter.Cluster.allows(expression.NewExpressionParams(cluster.GetLabelSet().Labels, nil), cache) {
		return false
	}

	return true
}

// NewGlobalRules creates and initializes a new empty list of global rules
func NewGlobalRules() *GlobalRules {
	return &GlobalRules{Rules: make(map[string][]*Rule, 0)}
}

func (globalRules *GlobalRules) addRule(rule *Rule) {
	if rule.FilterServices == nil {
		Debug.WithFields(log.Fields{
			"rule": rule,
		}).Panic("Only service filters currently supported in rules")
	}
	for _, action := range rule.Actions {
		if rulesList, ok := globalRules.Rules[action.Type]; ok {
			globalRules.Rules[action.Type] = append(rulesList, rule)
		} else {
			globalRules.Rules[action.Type] = []*Rule{rule}
		}
	}
}

func (rule *Rule) GetObjectType() SlingaObjectType {
	return TypePolicy
}

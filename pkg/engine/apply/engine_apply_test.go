package apply

import (
	"github.com/Aptomi/aptomi/pkg/config"
	"github.com/Aptomi/aptomi/pkg/engine/actual"
	"github.com/Aptomi/aptomi/pkg/engine/diff"
	"github.com/Aptomi/aptomi/pkg/engine/progress"
	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/event"
	"github.com/Aptomi/aptomi/pkg/external"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/lang/builder"
	"github.com/Aptomi/aptomi/pkg/plugin"
	"github.com/Aptomi/aptomi/pkg/plugin/fake"
	"github.com/Aptomi/aptomi/pkg/runtime"
	"github.com/Aptomi/aptomi/pkg/util"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	ResSuccess = iota
	ResError   = iota
)

func TestApplyComponentCreateSuccess(t *testing.T) {
	// resolve empty policy
	empty := newTestData(t, builder.NewPolicyBuilder())
	actualState := empty.resolution()

	// resolve full policy
	desired := newTestData(t, makePolicyBuilder())

	// apply changes
	applier := NewEngineApply(
		desired.policy(),
		desired.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		desired.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(desired.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// check actual state
	assert.Equal(t, 0, len(actualState.ComponentInstanceMap), "Actual state should be empty")

	// check that policy apply finished with expected results
	actualState = applyAndCheck(t, applier, ResSuccess, 0, "Successfully resolved")

	// check that actual state got updated
	assert.Equal(t, 2, len(actualState.ComponentInstanceMap), "Actual state should not be empty after apply()")
}

func TestApplyComponentCreateFailure(t *testing.T) {
	checkApplyComponentCreateFail(t, false)
}

func TestApplyComponentCreatePanic(t *testing.T) {
	checkApplyComponentCreateFail(t, true)
}

func checkApplyComponentCreateFail(t *testing.T, failAsPanic bool) {
	// resolve empty policy
	empty := newTestData(t, builder.NewPolicyBuilder())
	actualState := empty.resolution()

	// resolve full policy
	desired := newTestData(t, makePolicyBuilder())

	// process all actions (and make component fail deployment)
	applier := NewEngineApply(
		desired.policy(),
		desired.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		desired.external(),
		mockRegistryFailOnComponent(failAsPanic, desired.policy().GetObjectsByKind(lang.ServiceObject.Kind)[0].(*lang.Service).Components[0].Name),
		diff.NewPolicyResolutionDiff(desired.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)
	// check actual state
	assert.Equal(t, 0, len(actualState.ComponentInstanceMap), "Actual state should be empty")

	// check for errors
	actualState = applyAndCheck(t, applier, ResError, 1, "failed by plugin mock for component")

	// check that actual state got updated (service component exists, but no child components got deployed)
	assert.Equal(t, 1, len(actualState.ComponentInstanceMap), "Actual state should be correctly updated by apply()")
}

func TestDiffHasUpdatedComponentsAndCheckTimes(t *testing.T) {
	/*
		Step 1: actual = empty, desired = test policy, check = kafka update/create times
	*/

	// Create initial empty policy & resolution data
	empty := newTestData(t, builder.NewPolicyBuilder())
	actualState := empty.resolution()

	// Generate policy and resolve all dependencies in policy
	desired := newTestData(t, makePolicyBuilder())

	// Apply to update component times in actual state
	applier := NewEngineApply(
		desired.policy(),
		desired.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		desired.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(desired.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// Check that policy apply finished with expected results
	actualState = applyAndCheck(t, applier, ResSuccess, 0, "Successfully resolved")

	// Get key to a component
	cluster := desired.policy().GetObjectsByKind(lang.ClusterObject.Kind)[0].(*lang.Cluster)
	contract := desired.policy().GetObjectsByKind(lang.ContractObject.Kind)[0].(*lang.Contract)
	service := desired.policy().GetObjectsByKind(lang.ServiceObject.Kind)[0].(*lang.Service)
	key := resolve.NewComponentInstanceKey(cluster, contract, contract.Contexts[0], nil, service, service.Components[0])
	keyService := key.GetParentServiceKey()

	// Check creation/update times
	times1 := getTimes(t, key.GetKey(), actualState)
	assert.WithinDuration(t, time.Now(), times1.created, time.Second, "Creation time should be initialized correctly")
	assert.Equal(t, times1.updated, times1.updated, "Update time should be equal to creation time")

	/*
		Step 2: desired = add a dependency, check = component update/create times remained the same in actual state
	*/

	// Sleep a little bit to introduce time delay
	time.Sleep(25 * time.Millisecond)

	// Add another dependency, resolve, calculate difference against prev resolution data
	desiredNext := newTestData(t, makePolicyBuilder())
	dependencyNew := desiredNext.pBuilder.AddDependency(desiredNext.pBuilder.AddUser(), contract)
	dependencyNew.Labels["param"] = "value1"

	assert.Contains(t, desiredNext.resolution().GetDependencyInstanceMap(), runtime.KeyForStorable(dependencyNew), "Additional dependency should be resolved successfully")

	// Apply to update component times in actual state
	applier = NewEngineApply(
		desiredNext.policy(),
		desiredNext.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		desiredNext.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(desiredNext.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// Check that policy apply finished with expected results
	actualState = applyAndCheck(t, applier, ResSuccess, 0, "Successfully resolved")

	// Check creation/update times
	times2 := getTimes(t, key.GetKey(), actualState)
	assert.Equal(t, times1.created, times2.created, "Creation time should be carried over to remain the same")
	assert.Equal(t, times1.updated, times2.updated, "Update time should be carried over to remain the same")

	/*
		Step 3: desired = update user label, check = component update time changed
	*/
	componentTimes := getTimes(t, key.GetKey(), actualState)
	serviceTimes := getTimes(t, keyService.GetKey(), actualState)

	// Sleep a little bit to introduce time delay
	time.Sleep(25 * time.Millisecond)

	// Update labels, re-evaluate and see that component instance has changed
	desiredNextAfterUpdate := newTestData(t, desiredNext.pBuilder)
	for _, dependency := range desiredNextAfterUpdate.policy().GetObjectsByKind(lang.DependencyObject.Kind) {
		dependency.(*lang.Dependency).Labels["param"] = "value2"
	}

	// Apply to update component times in actual state
	applier = NewEngineApply(
		desiredNextAfterUpdate.policy(),
		desiredNextAfterUpdate.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		desiredNextAfterUpdate.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(desiredNextAfterUpdate.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// Check that policy apply finished with expected results
	actualState = applyAndCheck(t, applier, ResSuccess, 0, "Successfully resolved")

	// Check creation/update times for component
	componentTimesUpdated := getTimes(t, key.GetKey(), actualState)
	assert.Equal(t, componentTimes.created, componentTimesUpdated.created, "Creation time for component should be carried over to remain the same")
	assert.True(t, componentTimesUpdated.updated.After(componentTimes.updated), "Update time for component should be changed")

	// Check creation/update times for service
	serviceTimesUpdated := getTimes(t, keyService.GetKey(), actualState)
	assert.Equal(t, serviceTimes.created, serviceTimesUpdated.created, "Creation time for parent service should be carried over to remain the same")
	assert.True(t, serviceTimesUpdated.updated.After(serviceTimes.updated), "Update time for parent service should be changed")
}

func TestDeletePolicyObjectsWhileComponentInstancesAreStilRunningFails(t *testing.T) {
	// Start with empty actual state & empty policy
	empty := newTestData(t, builder.NewPolicyBuilder())
	actualState := empty.resolution()
	assert.Equal(t, 0, len(empty.resolution().ComponentInstanceMap), "Initial state should not have any components")
	assert.Equal(t, 0, len(actualState.ComponentInstanceMap), "Actual state should not have any components at this point")

	// Generate policy
	generated := newTestData(t, makePolicyBuilder())
	assert.Equal(t, 2, len(generated.resolution().ComponentInstanceMap), "Desired state should not be empty")

	// Run apply to update actual state
	applier := NewEngineApply(
		generated.policy(),
		generated.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		generated.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(generated.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// Check that policy apply finished with expected results
	actualState = applyAndCheck(t, applier, ResSuccess, 0, "Successfully resolved")
	assert.Equal(t, 2, len(actualState.ComponentInstanceMap), "Actual state should have populated with components at this point")

	// Reset policy back to empty
	reset := newTestData(t, builder.NewPolicyBuilder())

	// Run apply to update actual state
	applierNext := NewEngineApply(
		reset.policy(),
		reset.resolution(),
		actualState,
		actual.NewNoOpActionStateUpdater(),
		generated.external(),
		mockRegistryFailOnComponent(false),
		diff.NewPolicyResolutionDiff(reset.resolution(), actualState).Actions,
		event.NewLog("test-apply", false),
		progress.NewNoop(),
	)

	// delete/detach, delete/detach, endpoints/endpoints - 6 actions failed in total
	actualState = applyAndCheck(t, applierNext, ResError, 6, "error while applying action")
	assert.Equal(t, 2, len(actualState.ComponentInstanceMap), "Actual state should be intact after actions failing")
}

/*
	Helpers
*/

// Utility data structure for creating & resolving policy via builder in unit tests
type testData struct {
	t        *testing.T
	pBuilder *builder.PolicyBuilder
	resolved *resolve.PolicyResolution
}

func newTestData(t *testing.T, pBuilder *builder.PolicyBuilder) *testData {
	return &testData{t: t, pBuilder: pBuilder}
}

func (td *testData) policy() *lang.Policy {
	return td.pBuilder.Policy()
}

func (td *testData) resolution() *resolve.PolicyResolution {
	if td.resolved == nil {
		td.resolved = resolvePolicy(td.t, td.pBuilder)
	}
	return td.resolved
}

func (td *testData) external() *external.Data {
	return td.pBuilder.External()
}

func makePolicyBuilder() *builder.PolicyBuilder {
	b := builder.NewPolicyBuilder()

	// create a service
	service := b.AddService()
	b.AddServiceComponent(service,
		b.CodeComponent(
			util.NestedParameterMap{
				"param":   "{{ .Labels.param }}",
				"cluster": "{{ .Labels.cluster }}",
			},
			nil,
		),
	)
	contract := b.AddContract(service, b.CriteriaTrue())

	// add rule to set cluster
	clusterObj := b.AddCluster()
	b.AddRule(b.CriteriaTrue(), b.RuleActions(lang.NewLabelOperationsSetSingleLabel(lang.LabelCluster, clusterObj.Name)))

	// add dependency
	dependency := b.AddDependency(b.AddUser(), contract)
	dependency.Labels["param"] = "value1"

	return b
}

func resolvePolicy(t *testing.T, b *builder.PolicyBuilder) *resolve.PolicyResolution {
	t.Helper()
	eventLog := event.NewLog("test-resolve", false)
	resolver := resolve.NewPolicyResolver(b.Policy(), b.External(), eventLog)
	result, err := resolver.ResolveAllDependencies()
	if !assert.NoError(t, err, "Policy should be resolved without errors") {
		hook := &event.HookConsole{}
		eventLog.Save(hook)
		t.FailNow()
	}

	return result
}

func applyAndCheck(t *testing.T, apply *EngineApply, expectedResult int, errorCnt int, expectedMessage string) *resolve.PolicyResolution {
	t.Helper()
	actualState, err := apply.Apply()

	if !assert.Equal(t, expectedResult != ResError, err == nil, "Apply status (success vs. error)") {
		// print log into stdout and exit
		hook := &event.HookConsole{}
		apply.eventLog.Save(hook)
		t.FailNow()
	}

	if expectedResult == ResError {
		// check for error messages
		verifier := event.NewLogVerifier(expectedMessage, expectedResult == ResError)
		apply.eventLog.Save(verifier)
		if !assert.Equal(t, errorCnt, verifier.MatchedErrorsCount(), "Apply event log should have correct number of messages containing words: "+expectedMessage) {
			hook := &event.HookConsole{}
			apply.eventLog.Save(hook)
			t.FailNow()
		}
	}
	return actualState
}

type componentTimes struct {
	created time.Time
	updated time.Time
}

func getTimes(t *testing.T, key string, u2 *resolve.PolicyResolution) componentTimes {
	t.Helper()
	return componentTimes{
		created: getInstanceInternal(t, key, u2).CreatedAt,
		updated: getInstanceInternal(t, key, u2).UpdatedAt,
	}
}

func getInstanceInternal(t *testing.T, key string, resolution *resolve.PolicyResolution) *resolve.ComponentInstance {
	t.Helper()
	instance, ok := resolution.ComponentInstanceMap[key]
	if !assert.True(t, ok, "Component instance exists in resolution data: "+key) {
		t.FailNow()
	}
	return instance
}

func mockRegistryFailOnComponent(failAsPanic bool, failComponents ...string) plugin.Registry {
	clusterTypes := make(map[string]plugin.ClusterPluginConstructor)
	codeTypes := make(map[string]map[string]plugin.CodePluginConstructor)
	postProcessPlugins := make([]plugin.PostProcessPlugin, 0)

	clusterTypes["kubernetes"] = func(cluster *lang.Cluster, cfg config.Plugins) (plugin.ClusterPlugin, error) {
		return fake.NewNoOpClusterPlugin(0), nil
	}

	codeTypes["kubernetes"] = make(map[string]plugin.CodePluginConstructor)
	codeTypes["kubernetes"]["helm"] = func(cluster plugin.ClusterPlugin, cfg config.Plugins) (plugin.CodePlugin, error) {
		return fake.NewFailCodePlugin(failComponents, failAsPanic), nil
	}

	return plugin.NewRegistry(config.Plugins{}, clusterTypes, codeTypes, postProcessPlugins)
}

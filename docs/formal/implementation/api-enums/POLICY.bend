# FROZEN declaration-derived policy. Independent of Valid method bodies.
# Policy edits require explicit human review; --refresh NEVER changes this file.
import Base
import ./helpers.bend as H

def openaiapi_AdditionalToolsType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 100n, 100n, 105n, 116n, 105n, 111n, 110n, 97n, 108n, 95n, 116n, 111n, 111n, 108n, 115n]]

def openaiapi_AdditionalToolsType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_AdditionalToolsType_Valid_constants(), x)

def openaiapi_AdditionalToolsItemParamRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 101n, 118n, 101n, 108n, 111n, 112n, 101n, 114n]]

def openaiapi_AdditionalToolsItemParamRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_AdditionalToolsItemParamRole_Valid_constants(), x)

def openaiapi_AdditionalToolsItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 100n, 100n, 105n, 116n, 105n, 111n, 110n, 97n, 108n, 95n, 116n, 111n, 111n, 108n, 115n]]

def openaiapi_AdditionalToolsItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_AdditionalToolsItemParamType_Valid_constants(), x)

def openaiapi_ApplyPatchCallOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ApplyPatchCallOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCallOutputStatus_Valid_constants(), x)

def openaiapi_ApplyPatchCallOutputStatusParam_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ApplyPatchCallOutputStatusParam_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCallOutputStatusParam_Valid_constants(), x)

def openaiapi_ApplyPatchCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ApplyPatchCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCallStatus_Valid_constants(), x)

def openaiapi_ApplyPatchCallStatusParam_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ApplyPatchCallStatusParam_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCallStatusParam_Valid_constants(), x)

def openaiapi_ApplyPatchCreateFileOperationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 114n, 101n, 97n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchCreateFileOperationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCreateFileOperationType_Valid_constants(), x)

def openaiapi_ApplyPatchCreateFileOperationParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 114n, 101n, 97n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchCreateFileOperationParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchCreateFileOperationParamType_Valid_constants(), x)

def openaiapi_ApplyPatchDeleteFileOperationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 101n, 108n, 101n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchDeleteFileOperationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchDeleteFileOperationType_Valid_constants(), x)

def openaiapi_ApplyPatchDeleteFileOperationParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 101n, 108n, 101n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchDeleteFileOperationParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchDeleteFileOperationParamType_Valid_constants(), x)

def openaiapi_ApplyPatchToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ApplyPatchToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchToolCallType_Valid_constants(), x)

def openaiapi_ApplyPatchToolCallItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ApplyPatchToolCallItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchToolCallItemParamType_Valid_constants(), x)

def openaiapi_ApplyPatchToolCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ApplyPatchToolCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchToolCallOutputType_Valid_constants(), x)

def openaiapi_ApplyPatchToolCallOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ApplyPatchToolCallOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchToolCallOutputItemParamType_Valid_constants(), x)

def openaiapi_ApplyPatchToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n]]

def openaiapi_ApplyPatchToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchToolParamType_Valid_constants(), x)

def openaiapi_ApplyPatchUpdateFileOperationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 112n, 100n, 97n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchUpdateFileOperationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchUpdateFileOperationType_Valid_constants(), x)

def openaiapi_ApplyPatchUpdateFileOperationParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 112n, 100n, 97n, 116n, 101n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_ApplyPatchUpdateFileOperationParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApplyPatchUpdateFileOperationParamType_Valid_constants(), x)

def openaiapi_ApproximateLocationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 114n, 111n, 120n, 105n, 109n, 97n, 116n, 101n]]

def openaiapi_ApproximateLocationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ApproximateLocationType_Valid_constants(), x)

def openaiapi_AutoCodeInterpreterToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n]]

def openaiapi_AutoCodeInterpreterToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_AutoCodeInterpreterToolParamType_Valid_constants(), x)

def openaiapi_CallableToolAllowedCaller_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 114n, 101n, 99n, 116n], [112n, 114n, 111n, 103n, 114n, 97n, 109n, 109n, 97n, 116n, 105n, 99n]]

def openaiapi_CallableToolAllowedCaller_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CallableToolAllowedCaller_Valid_constants(), x)

def openaiapi_ClickButtonType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 99n, 107n], [102n, 111n, 114n, 119n, 97n, 114n, 100n], [108n, 101n, 102n, 116n], [114n, 105n, 103n, 104n, 116n], [119n, 104n, 101n, 101n, 108n]]

def openaiapi_ClickButtonType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ClickButtonType_Valid_constants(), x)

def openaiapi_ClickParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 108n, 105n, 99n, 107n]]

def openaiapi_ClickParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ClickParamType_Valid_constants(), x)

def openaiapi_CodeInterpreterOutputImageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n]]

def openaiapi_CodeInterpreterOutputImageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CodeInterpreterOutputImageType_Valid_constants(), x)

def openaiapi_CodeInterpreterOutputLogsType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 103n, 115n]]

def openaiapi_CodeInterpreterOutputLogsType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CodeInterpreterOutputLogsType_Valid_constants(), x)

def openaiapi_CodeInterpreterToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n]]

def openaiapi_CodeInterpreterToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CodeInterpreterToolType_Valid_constants(), x)

def openaiapi_CodeInterpreterToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n], [105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 105n, 110n, 103n]]

def openaiapi_CodeInterpreterToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CodeInterpreterToolCallStatus_Valid_constants(), x)

def openaiapi_CodeInterpreterToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_CodeInterpreterToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CodeInterpreterToolCallType_Valid_constants(), x)

def openaiapi_CompactionBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 97n, 99n, 116n, 105n, 111n, 110n]]

def openaiapi_CompactionBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CompactionBodyType_Valid_constants(), x)

def openaiapi_CompactionSummaryItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 97n, 99n, 116n, 105n, 111n, 110n]]

def openaiapi_CompactionSummaryItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CompactionSummaryItemParamType_Valid_constants(), x)

def openaiapi_CompactionTriggerItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 97n, 99n, 116n, 105n, 111n, 110n, 95n, 116n, 114n, 105n, 103n, 103n, 101n, 114n]]

def openaiapi_CompactionTriggerItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CompactionTriggerItemParamType_Valid_constants(), x)

def openaiapi_ComparisonFilterType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 113n], [103n, 116n], [103n, 116n, 101n], [105n, 110n], [108n, 116n], [108n, 116n, 101n], [110n, 101n], [110n, 105n, 110n]]

def openaiapi_ComparisonFilterType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComparisonFilterType_Valid_constants(), x)

def openaiapi_CompoundFilterType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 110n, 100n], [111n, 114n]]

def openaiapi_CompoundFilterType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CompoundFilterType_Valid_constants(), x)

def openaiapi_ComputerCallOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ComputerCallOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerCallOutputItemParamType_Valid_constants(), x)

def openaiapi_ComputerCallOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ComputerCallOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerCallOutputStatus_Valid_constants(), x)

def openaiapi_ComputerEnvironment_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 114n, 111n, 119n, 115n, 101n, 114n], [108n, 105n, 110n, 117n, 120n], [109n, 97n, 99n], [117n, 98n, 117n, 110n, 116n, 117n], [119n, 105n, 110n, 100n, 111n, 119n, 115n]]

def openaiapi_ComputerEnvironment_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerEnvironment_Valid_constants(), x)

def openaiapi_ComputerScreenshotImageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 115n, 99n, 114n, 101n, 101n, 110n, 115n, 104n, 111n, 116n]]

def openaiapi_ComputerScreenshotImageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerScreenshotImageType_Valid_constants(), x)

def openaiapi_ComputerToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n]]

def openaiapi_ComputerToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolType_Valid_constants(), x)

def openaiapi_ComputerToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ComputerToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolCallStatus_Valid_constants(), x)

def openaiapi_ComputerToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ComputerToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolCallType_Valid_constants(), x)

def openaiapi_ComputerToolCallOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ComputerToolCallOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolCallOutputStatus_Valid_constants(), x)

def openaiapi_ComputerToolCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ComputerToolCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolCallOutputType_Valid_constants(), x)

def openaiapi_ComputerToolCallOutputResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ComputerToolCallOutputResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerToolCallOutputResourceType_Valid_constants(), x)

def openaiapi_ComputerUsePreviewToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 117n, 115n, 101n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n]]

def openaiapi_ComputerUsePreviewToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ComputerUsePreviewToolType_Valid_constants(), x)

def openaiapi_ContainerAutoParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 97n, 117n, 116n, 111n]]

def openaiapi_ContainerAutoParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerAutoParamType_Valid_constants(), x)

def openaiapi_ContainerFileCitationBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 102n, 105n, 108n, 101n, 95n, 99n, 105n, 116n, 97n, 116n, 105n, 111n, 110n]]

def openaiapi_ContainerFileCitationBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerFileCitationBodyType_Valid_constants(), x)

def openaiapi_ContainerMemoryLimit_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[49n, 54n, 103n], [49n, 103n], [52n, 103n], [54n, 52n, 103n]]

def openaiapi_ContainerMemoryLimit_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerMemoryLimit_Valid_constants(), x)

def openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 108n, 108n, 111n, 119n, 108n, 105n, 115n, 116n]]

def openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid_constants(), x)

def openaiapi_ContainerNetworkPolicyDisabledParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 115n, 97n, 98n, 108n, 101n, 100n]]

def openaiapi_ContainerNetworkPolicyDisabledParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerNetworkPolicyDisabledParamType_Valid_constants(), x)

def openaiapi_ContainerReferenceParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def openaiapi_ContainerReferenceParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerReferenceParamType_Valid_constants(), x)

def openaiapi_ContainerReferenceResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def openaiapi_ContainerReferenceResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ContainerReferenceResourceType_Valid_constants(), x)

def openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 95n, 109n, 101n, 109n, 111n, 114n, 121n], [50n, 52n, 104n]]

def openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid_constants(), x)

def openaiapi_CreateResponsePromptCacheRetention_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 95n, 109n, 101n, 109n, 111n, 114n, 121n], [50n, 52n, 104n]]

def openaiapi_CreateResponsePromptCacheRetention_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CreateResponsePromptCacheRetention_Valid_constants(), x)

def openaiapi_CreateResponseTruncation_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [100n, 105n, 115n, 97n, 98n, 108n, 101n, 100n]]

def openaiapi_CreateResponseTruncation_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CreateResponseTruncation_Valid_constants(), x)

def openaiapi_CustomGrammarFormatParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[103n, 114n, 97n, 109n, 109n, 97n, 114n]]

def openaiapi_CustomGrammarFormatParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomGrammarFormatParamType_Valid_constants(), x)

def openaiapi_CustomTextFormatParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n]]

def openaiapi_CustomTextFormatParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomTextFormatParamType_Valid_constants(), x)

def openaiapi_CustomToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_CustomToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomToolCallType_Valid_constants(), x)

def openaiapi_CustomToolCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_CustomToolCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomToolCallOutputType_Valid_constants(), x)

def openaiapi_CustomToolCallOutputResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_CustomToolCallOutputResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomToolCallOutputResourceType_Valid_constants(), x)

def openaiapi_CustomToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n]]

def openaiapi_CustomToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_CustomToolParamType_Valid_constants(), x)

def openaiapi_DetailEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [104n, 105n, 103n, 104n], [108n, 111n, 119n], [111n, 114n, 105n, 103n, 105n, 110n, 97n, 108n]]

def openaiapi_DetailEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_DetailEnum_Valid_constants(), x)

def openaiapi_DirectToolCallCallerType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 114n, 101n, 99n, 116n]]

def openaiapi_DirectToolCallCallerType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_DirectToolCallCallerType_Valid_constants(), x)

def openaiapi_DirectToolCallCallerParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 114n, 101n, 99n, 116n]]

def openaiapi_DirectToolCallCallerParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_DirectToolCallCallerParamType_Valid_constants(), x)

def openaiapi_DoubleClickActionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 117n, 98n, 108n, 101n, 95n, 99n, 108n, 105n, 99n, 107n]]

def openaiapi_DoubleClickActionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_DoubleClickActionType_Valid_constants(), x)

def openaiapi_DragParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 114n, 97n, 103n]]

def openaiapi_DragParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_DragParamType_Valid_constants(), x)

def openaiapi_EasyInputMessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 115n, 115n, 105n, 115n, 116n, 97n, 110n, 116n], [100n, 101n, 118n, 101n, 108n, 111n, 112n, 101n, 114n], [115n, 121n, 115n, 116n, 101n, 109n], [117n, 115n, 101n, 114n]]

def openaiapi_EasyInputMessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_EasyInputMessageRole_Valid_constants(), x)

def openaiapi_EasyInputMessageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n]]

def openaiapi_EasyInputMessageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_EasyInputMessageType_Valid_constants(), x)

def openaiapi_FileCitationBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n, 95n, 99n, 105n, 116n, 97n, 116n, 105n, 111n, 110n]]

def openaiapi_FileCitationBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileCitationBodyType_Valid_constants(), x)

def openaiapi_FileDetailEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [104n, 105n, 103n, 104n], [108n, 111n, 119n]]

def openaiapi_FileDetailEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileDetailEnum_Valid_constants(), x)

def openaiapi_FileInputDetail_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [104n, 105n, 103n, 104n], [108n, 111n, 119n]]

def openaiapi_FileInputDetail_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileInputDetail_Valid_constants(), x)

def openaiapi_FilePathType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n, 95n, 112n, 97n, 116n, 104n]]

def openaiapi_FilePathType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FilePathType_Valid_constants(), x)

def openaiapi_FileSearchToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def openaiapi_FileSearchToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileSearchToolType_Valid_constants(), x)

def openaiapi_FileSearchToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n], [115n, 101n, 97n, 114n, 99n, 104n, 105n, 110n, 103n]]

def openaiapi_FileSearchToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileSearchToolCallStatus_Valid_constants(), x)

def openaiapi_FileSearchToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_FileSearchToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FileSearchToolCallType_Valid_constants(), x)

def openaiapi_FunctionCallItemStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionCallItemStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionCallItemStatus_Valid_constants(), x)

def openaiapi_FunctionCallOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_FunctionCallOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionCallOutputItemParamType_Valid_constants(), x)

def openaiapi_FunctionCallOutputStatusEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionCallOutputStatusEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionCallOutputStatusEnum_Valid_constants(), x)

def openaiapi_FunctionCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionCallStatus_Valid_constants(), x)

def openaiapi_FunctionShellCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_FunctionShellCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallType_Valid_constants(), x)

def openaiapi_FunctionShellCallItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_FunctionShellCallItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallItemParamType_Valid_constants(), x)

def openaiapi_FunctionShellCallItemStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionShellCallItemStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallItemStatus_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_FunctionShellCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputType_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputExitOutcomeType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 105n, 116n]]

def openaiapi_FunctionShellCallOutputExitOutcomeType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputExitOutcomeType_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 105n, 116n]]

def openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_FunctionShellCallOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputItemParamType_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputStatusEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionShellCallOutputStatusEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputStatusEnum_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 105n, 109n, 101n, 111n, 117n, 116n]]

def openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid_constants(), x)

def openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 105n, 109n, 101n, 111n, 117n, 116n]]

def openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid_constants(), x)

def openaiapi_FunctionShellCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionShellCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellCallStatus_Valid_constants(), x)

def openaiapi_FunctionShellToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n]]

def openaiapi_FunctionShellToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionShellToolParamType_Valid_constants(), x)

def openaiapi_FunctionToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n]]

def openaiapi_FunctionToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolType_Valid_constants(), x)

def openaiapi_FunctionToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolCallStatus_Valid_constants(), x)

def openaiapi_FunctionToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_FunctionToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolCallType_Valid_constants(), x)

def openaiapi_FunctionToolCallOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_FunctionToolCallOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolCallOutputStatus_Valid_constants(), x)

def openaiapi_FunctionToolCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_FunctionToolCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolCallOutputType_Valid_constants(), x)

def openaiapi_FunctionToolCallOutputResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_FunctionToolCallOutputResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolCallOutputResourceType_Valid_constants(), x)

def openaiapi_FunctionToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n]]

def openaiapi_FunctionToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_FunctionToolParamType_Valid_constants(), x)

def openaiapi_GrammarSyntax1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 97n, 114n, 107n], [114n, 101n, 103n, 101n, 120n]]

def openaiapi_GrammarSyntax1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_GrammarSyntax1_Valid_constants(), x)

def openaiapi_ImageDetail_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [104n, 105n, 103n, 104n], [108n, 111n, 119n], [111n, 114n, 105n, 103n, 105n, 110n, 97n, 108n]]

def openaiapi_ImageDetail_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageDetail_Valid_constants(), x)

def openaiapi_ImageGenActionEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [101n, 100n, 105n, 116n], [103n, 101n, 110n, 101n, 114n, 97n, 116n, 101n]]

def openaiapi_ImageGenActionEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenActionEnum_Valid_constants(), x)

def openaiapi_ImageGenToolBackground_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [111n, 112n, 97n, 113n, 117n, 101n], [116n, 114n, 97n, 110n, 115n, 112n, 97n, 114n, 101n, 110n, 116n]]

def openaiapi_ImageGenToolBackground_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolBackground_Valid_constants(), x)

def openaiapi_ImageGenToolModel1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[103n, 112n, 116n, 45n, 105n, 109n, 97n, 103n, 101n, 45n, 49n], [103n, 112n, 116n, 45n, 105n, 109n, 97n, 103n, 101n, 45n, 49n, 46n, 53n], [103n, 112n, 116n, 45n, 105n, 109n, 97n, 103n, 101n, 45n, 49n, 45n, 109n, 105n, 110n, 105n]]

def openaiapi_ImageGenToolModel1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolModel1_Valid_constants(), x)

def openaiapi_ImageGenToolModeration_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [108n, 111n, 119n]]

def openaiapi_ImageGenToolModeration_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolModeration_Valid_constants(), x)

def openaiapi_ImageGenToolOutputFormat_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[106n, 112n, 101n, 103n], [112n, 110n, 103n], [119n, 101n, 98n, 112n]]

def openaiapi_ImageGenToolOutputFormat_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolOutputFormat_Valid_constants(), x)

def openaiapi_ImageGenToolQuality_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [104n, 105n, 103n, 104n], [108n, 111n, 119n], [109n, 101n, 100n, 105n, 117n, 109n]]

def openaiapi_ImageGenToolQuality_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolQuality_Valid_constants(), x)

def openaiapi_ImageGenToolSize1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [49n, 48n, 50n, 52n, 120n, 49n, 48n, 50n, 52n], [49n, 48n, 50n, 52n, 120n, 49n, 53n, 51n, 54n], [49n, 53n, 51n, 54n, 120n, 49n, 48n, 50n, 52n]]

def openaiapi_ImageGenToolSize1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolSize1_Valid_constants(), x)

def openaiapi_ImageGenToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n]]

def openaiapi_ImageGenToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolType_Valid_constants(), x)

def openaiapi_ImageGenToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 110n, 103n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ImageGenToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolCallStatus_Valid_constants(), x)

def openaiapi_ImageGenToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ImageGenToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ImageGenToolCallType_Valid_constants(), x)

def openaiapi_IncludeEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 115n], [99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 117n, 114n, 108n], [102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 114n, 101n, 115n, 117n, 108n, 116n, 115n], [109n, 101n, 115n, 115n, 97n, 103n, 101n, 46n, 105n, 110n, 112n, 117n, 116n, 95n, 105n, 109n, 97n, 103n, 101n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 117n, 114n, 108n], [109n, 101n, 115n, 115n, 97n, 103n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n, 46n, 108n, 111n, 103n, 112n, 114n, 111n, 98n, 115n], [114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 46n, 101n, 110n, 99n, 114n, 121n, 112n, 116n, 101n, 100n, 95n, 99n, 111n, 110n, 116n, 101n, 110n, 116n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 97n, 99n, 116n, 105n, 111n, 110n, 46n, 115n, 111n, 117n, 114n, 99n, 101n, 115n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 114n, 101n, 115n, 117n, 108n, 116n, 115n]]

def openaiapi_IncludeEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_IncludeEnum_Valid_constants(), x)

def openaiapi_InlineSkillParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 108n, 105n, 110n, 101n]]

def openaiapi_InlineSkillParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InlineSkillParamType_Valid_constants(), x)

def openaiapi_InlineSkillSourceParamMediaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 105n, 99n, 97n, 116n, 105n, 111n, 110n, 47n, 122n, 105n, 112n]]

def openaiapi_InlineSkillSourceParamMediaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InlineSkillSourceParamMediaType_Valid_constants(), x)

def openaiapi_InlineSkillSourceParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 101n, 54n, 52n]]

def openaiapi_InlineSkillSourceParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InlineSkillSourceParamType_Valid_constants(), x)

def openaiapi_InputFidelity_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[104n, 105n, 103n, 104n], [108n, 111n, 119n]]

def openaiapi_InputFidelity_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputFidelity_Valid_constants(), x)

def openaiapi_InputFileContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_InputFileContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputFileContentType_Valid_constants(), x)

def openaiapi_InputFileContentParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 102n, 105n, 108n, 101n]]

def openaiapi_InputFileContentParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputFileContentParamType_Valid_constants(), x)

def openaiapi_InputImageContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 105n, 109n, 97n, 103n, 101n]]

def openaiapi_InputImageContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputImageContentType_Valid_constants(), x)

def openaiapi_InputImageContentParamAutoParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 105n, 109n, 97n, 103n, 101n]]

def openaiapi_InputImageContentParamAutoParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputImageContentParamAutoParamType_Valid_constants(), x)

def openaiapi_InputMessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 101n, 118n, 101n, 108n, 111n, 112n, 101n, 114n], [115n, 121n, 115n, 116n, 101n, 109n], [117n, 115n, 101n, 114n]]

def openaiapi_InputMessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputMessageRole_Valid_constants(), x)

def openaiapi_InputMessageStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_InputMessageStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputMessageStatus_Valid_constants(), x)

def openaiapi_InputMessageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n]]

def openaiapi_InputMessageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputMessageType_Valid_constants(), x)

def openaiapi_InputTextContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_InputTextContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputTextContentType_Valid_constants(), x)

def openaiapi_InputTextContentParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_InputTextContentParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_InputTextContentParamType_Valid_constants(), x)

def openaiapi_ItemReferenceParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 116n, 101n, 109n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def openaiapi_ItemReferenceParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ItemReferenceParamType_Valid_constants(), x)

def openaiapi_KeyPressActionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[107n, 101n, 121n, 112n, 114n, 101n, 115n, 115n]]

def openaiapi_KeyPressActionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_KeyPressActionType_Valid_constants(), x)

def openaiapi_LocalEnvironmentParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 99n, 97n, 108n]]

def openaiapi_LocalEnvironmentParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalEnvironmentParamType_Valid_constants(), x)

def openaiapi_LocalEnvironmentResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 99n, 97n, 108n]]

def openaiapi_LocalEnvironmentResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalEnvironmentResourceType_Valid_constants(), x)

def openaiapi_LocalShellExecActionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 101n, 99n]]

def openaiapi_LocalShellExecActionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellExecActionType_Valid_constants(), x)

def openaiapi_LocalShellToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_LocalShellToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellToolCallStatus_Valid_constants(), x)

def openaiapi_LocalShellToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 99n, 97n, 108n, 95n, 115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_LocalShellToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellToolCallType_Valid_constants(), x)

def openaiapi_LocalShellToolCallOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_LocalShellToolCallOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellToolCallOutputStatus_Valid_constants(), x)

def openaiapi_LocalShellToolCallOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 99n, 97n, 108n, 95n, 115n, 104n, 101n, 108n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_LocalShellToolCallOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellToolCallOutputType_Valid_constants(), x)

def openaiapi_LocalShellToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[108n, 111n, 99n, 97n, 108n, 95n, 115n, 104n, 101n, 108n, 108n]]

def openaiapi_LocalShellToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_LocalShellToolParamType_Valid_constants(), x)

def openaiapi_MCPApprovalRequestType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n, 95n, 97n, 112n, 112n, 114n, 111n, 118n, 97n, 108n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n]]

def openaiapi_MCPApprovalRequestType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPApprovalRequestType_Valid_constants(), x)

def openaiapi_MCPApprovalResponseType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n, 95n, 97n, 112n, 112n, 114n, 111n, 118n, 97n, 108n, 95n, 114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n]]

def openaiapi_MCPApprovalResponseType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPApprovalResponseType_Valid_constants(), x)

def openaiapi_MCPApprovalResponseResourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n, 95n, 97n, 112n, 112n, 114n, 111n, 118n, 97n, 108n, 95n, 114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n]]

def openaiapi_MCPApprovalResponseResourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPApprovalResponseResourceType_Valid_constants(), x)

def openaiapi_MCPListToolsType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n, 95n, 108n, 105n, 115n, 116n, 95n, 116n, 111n, 111n, 108n, 115n]]

def openaiapi_MCPListToolsType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPListToolsType_Valid_constants(), x)

def openaiapi_MCPToolConnectorId_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 100n, 114n, 111n, 112n, 98n, 111n, 120n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 103n, 109n, 97n, 105n, 108n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 103n, 111n, 111n, 103n, 108n, 101n, 99n, 97n, 108n, 101n, 110n, 100n, 97n, 114n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 103n, 111n, 111n, 103n, 108n, 101n, 100n, 114n, 105n, 118n, 101n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 109n, 105n, 99n, 114n, 111n, 115n, 111n, 102n, 116n, 116n, 101n, 97n, 109n, 115n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 111n, 117n, 116n, 108n, 111n, 111n, 107n, 99n, 97n, 108n, 101n, 110n, 100n, 97n, 114n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 111n, 117n, 116n, 108n, 111n, 111n, 107n, 101n, 109n, 97n, 105n, 108n], [99n, 111n, 110n, 110n, 101n, 99n, 116n, 111n, 114n, 95n, 115n, 104n, 97n, 114n, 101n, 112n, 111n, 105n, 110n, 116n]]

def openaiapi_MCPToolConnectorId_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPToolConnectorId_Valid_constants(), x)

def openaiapi_MCPToolRequireApproval1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 108n, 119n, 97n, 121n, 115n], [110n, 101n, 118n, 101n, 114n]]

def openaiapi_MCPToolRequireApproval1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPToolRequireApproval1_Valid_constants(), x)

def openaiapi_MCPToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n]]

def openaiapi_MCPToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPToolType_Valid_constants(), x)

def openaiapi_MCPToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_MCPToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPToolCallType_Valid_constants(), x)

def openaiapi_MCPToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 97n, 108n, 108n, 105n, 110n, 103n], [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_MCPToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MCPToolCallStatus_Valid_constants(), x)

def openaiapi_MessagePhase_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 109n, 101n, 110n, 116n, 97n, 114n, 121n], [102n, 105n, 110n, 97n, 108n, 95n, 97n, 110n, 115n, 119n, 101n, 114n]]

def openaiapi_MessagePhase_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MessagePhase_Valid_constants(), x)

def openaiapi_MessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 115n, 115n, 105n, 115n, 116n, 97n, 110n, 116n], [99n, 114n, 105n, 116n, 105n, 99n], [100n, 101n, 118n, 101n, 108n, 111n, 112n, 101n, 114n], [100n, 105n, 115n, 99n, 114n, 105n, 109n, 105n, 110n, 97n, 116n, 111n, 114n], [115n, 121n, 115n, 116n, 101n, 109n], [116n, 111n, 111n, 108n], [117n, 110n, 107n, 110n, 111n, 119n, 110n], [117n, 115n, 101n, 114n]]

def openaiapi_MessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MessageRole_Valid_constants(), x)

def openaiapi_ModelIdsResponses1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 45n, 117n, 115n, 101n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 45n, 117n, 115n, 101n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 51n, 45n, 49n, 49n], [103n, 112n, 116n, 45n, 53n, 46n, 49n, 45n, 99n, 111n, 100n, 101n, 120n, 45n, 109n, 97n, 120n], [103n, 112n, 116n, 45n, 53n, 45n, 99n, 111n, 100n, 101n, 120n], [103n, 112n, 116n, 45n, 53n, 45n, 112n, 114n, 111n], [103n, 112n, 116n, 45n, 53n, 45n, 112n, 114n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 49n, 48n, 45n, 48n, 54n], [111n, 49n, 45n, 112n, 114n, 111n], [111n, 49n, 45n, 112n, 114n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 51n, 45n, 49n, 57n], [111n, 51n, 45n, 100n, 101n, 101n, 112n, 45n, 114n, 101n, 115n, 101n, 97n, 114n, 99n, 104n], [111n, 51n, 45n, 100n, 101n, 101n, 112n, 45n, 114n, 101n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 54n, 45n, 50n, 54n], [111n, 51n, 45n, 112n, 114n, 111n], [111n, 51n, 45n, 112n, 114n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 54n, 45n, 49n, 48n], [111n, 52n, 45n, 109n, 105n, 110n, 105n, 45n, 100n, 101n, 101n, 112n, 45n, 114n, 101n, 115n, 101n, 97n, 114n, 99n, 104n], [111n, 52n, 45n, 109n, 105n, 110n, 105n, 45n, 100n, 101n, 101n, 112n, 45n, 114n, 101n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 54n, 45n, 50n, 54n]]

def openaiapi_ModelIdsResponses1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModelIdsResponses1_Valid_constants(), x)

def openaiapi_ModelIdsShared1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 104n, 97n, 116n, 103n, 112n, 116n, 45n, 52n, 111n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [99n, 111n, 100n, 101n, 120n, 45n, 109n, 105n, 110n, 105n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 48n, 49n, 50n, 53n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 48n, 51n, 48n, 49n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 48n, 54n, 49n, 51n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 49n, 49n, 48n, 54n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 49n, 54n, 107n], [103n, 112n, 116n, 45n, 51n, 46n, 53n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 49n, 54n, 107n, 45n, 48n, 54n, 49n, 51n], [103n, 112n, 116n, 45n, 52n], [103n, 112n, 116n, 45n, 52n, 45n, 48n, 49n, 50n, 53n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 45n, 48n, 51n, 49n, 52n], [103n, 112n, 116n, 45n, 52n, 45n, 48n, 54n, 49n, 51n], [103n, 112n, 116n, 45n, 52n, 46n, 49n], [103n, 112n, 116n, 45n, 52n, 45n, 49n, 49n, 48n, 54n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 46n, 49n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 52n, 45n, 49n, 52n], [103n, 112n, 116n, 45n, 52n, 46n, 49n, 45n, 109n, 105n, 110n, 105n], [103n, 112n, 116n, 45n, 52n, 46n, 49n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 52n, 45n, 49n, 52n], [103n, 112n, 116n, 45n, 52n, 46n, 49n, 45n, 110n, 97n, 110n, 111n], [103n, 112n, 116n, 45n, 52n, 46n, 49n, 45n, 110n, 97n, 110n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 52n, 45n, 49n, 52n], [103n, 112n, 116n, 45n, 52n, 45n, 51n, 50n, 107n], [103n, 112n, 116n, 45n, 52n, 45n, 51n, 50n, 107n, 45n, 48n, 51n, 49n, 52n], [103n, 112n, 116n, 45n, 52n, 45n, 51n, 50n, 107n, 45n, 48n, 54n, 49n, 51n], [103n, 112n, 116n, 45n, 52n, 45n, 116n, 117n, 114n, 98n, 111n], [103n, 112n, 116n, 45n, 52n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 52n, 45n, 48n, 57n], [103n, 112n, 116n, 45n, 52n, 45n, 116n, 117n, 114n, 98n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 45n, 118n, 105n, 115n, 105n, 111n, 110n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 111n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 53n, 45n, 49n, 51n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 56n, 45n, 48n, 54n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 49n, 45n, 50n, 48n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 48n, 45n, 48n, 49n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 50n, 45n, 49n, 55n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 54n, 45n, 48n, 51n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 55n, 45n, 49n, 56n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n, 45n, 97n, 117n, 100n, 105n, 111n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 50n, 45n, 49n, 55n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n, 45n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 109n, 105n, 110n, 105n, 45n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 51n, 45n, 49n, 49n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [103n, 112n, 116n, 45n, 52n, 111n, 45n, 115n, 101n, 97n, 114n, 99n, 104n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 51n, 45n, 49n, 49n], [103n, 112n, 116n, 45n, 53n], [103n, 112n, 116n, 45n, 53n, 46n, 49n], [103n, 112n, 116n, 45n, 53n, 46n, 49n, 45n, 50n, 48n, 50n, 53n, 45n, 49n, 49n, 45n, 49n, 51n], [103n, 112n, 116n, 45n, 53n, 46n, 49n, 45n, 99n, 104n, 97n, 116n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [103n, 112n, 116n, 45n, 53n, 46n, 49n, 45n, 99n, 111n, 100n, 101n, 120n], [103n, 112n, 116n, 45n, 53n, 46n, 49n, 45n, 109n, 105n, 110n, 105n], [103n, 112n, 116n, 45n, 53n, 46n, 50n], [103n, 112n, 116n, 45n, 53n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 56n, 45n, 48n, 55n], [103n, 112n, 116n, 45n, 53n, 46n, 50n, 45n, 50n, 48n, 50n, 53n, 45n, 49n, 50n, 45n, 49n, 49n], [103n, 112n, 116n, 45n, 53n, 46n, 50n, 45n, 99n, 104n, 97n, 116n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [103n, 112n, 116n, 45n, 53n, 46n, 50n, 45n, 112n, 114n, 111n], [103n, 112n, 116n, 45n, 53n, 46n, 50n, 45n, 112n, 114n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 49n, 50n, 45n, 49n, 49n], [103n, 112n, 116n, 45n, 53n, 46n, 51n, 45n, 99n, 104n, 97n, 116n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [103n, 112n, 116n, 45n, 53n, 46n, 52n], [103n, 112n, 116n, 45n, 53n, 46n, 52n, 45n, 109n, 105n, 110n, 105n], [103n, 112n, 116n, 45n, 53n, 46n, 52n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 54n, 45n, 48n, 51n, 45n, 49n, 55n], [103n, 112n, 116n, 45n, 53n, 46n, 52n, 45n, 110n, 97n, 110n, 111n], [103n, 112n, 116n, 45n, 53n, 46n, 52n, 45n, 110n, 97n, 110n, 111n, 45n, 50n, 48n, 50n, 54n, 45n, 48n, 51n, 45n, 49n, 55n], [103n, 112n, 116n, 45n, 53n, 46n, 53n], [103n, 112n, 116n, 45n, 53n, 46n, 54n, 45n, 108n, 117n, 110n, 97n], [103n, 112n, 116n, 45n, 53n, 46n, 54n, 45n, 115n, 111n, 108n], [103n, 112n, 116n, 45n, 53n, 46n, 54n, 45n, 116n, 101n, 114n, 114n, 97n], [103n, 112n, 116n, 45n, 53n, 45n, 99n, 104n, 97n, 116n, 45n, 108n, 97n, 116n, 101n, 115n, 116n], [103n, 112n, 116n, 45n, 53n, 45n, 109n, 105n, 110n, 105n], [103n, 112n, 116n, 45n, 53n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 56n, 45n, 48n, 55n], [103n, 112n, 116n, 45n, 53n, 45n, 110n, 97n, 110n, 111n], [103n, 112n, 116n, 45n, 53n, 45n, 110n, 97n, 110n, 111n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 56n, 45n, 48n, 55n], [111n, 49n], [111n, 49n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 50n, 45n, 49n, 55n], [111n, 49n, 45n, 109n, 105n, 110n, 105n], [111n, 49n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 57n, 45n, 49n, 50n], [111n, 49n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [111n, 49n, 45n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 45n, 50n, 48n, 50n, 52n, 45n, 48n, 57n, 45n, 49n, 50n], [111n, 51n], [111n, 51n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 52n, 45n, 49n, 54n], [111n, 51n, 45n, 109n, 105n, 110n, 105n], [111n, 51n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 49n, 45n, 51n, 49n], [111n, 52n, 45n, 109n, 105n, 110n, 105n], [111n, 52n, 45n, 109n, 105n, 110n, 105n, 45n, 50n, 48n, 50n, 53n, 45n, 48n, 52n, 45n, 49n, 54n]]

def openaiapi_ModelIdsShared1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModelIdsShared1_Valid_constants(), x)

def openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 95n, 109n, 101n, 109n, 111n, 114n, 121n], [50n, 52n, 104n]]

def openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid_constants(), x)

def openaiapi_ModerationErrorBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 114n, 114n, 111n, 114n]]

def openaiapi_ModerationErrorBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModerationErrorBodyType_Valid_constants(), x)

def openaiapi_ModerationInputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n], [116n, 101n, 120n, 116n]]

def openaiapi_ModerationInputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModerationInputType_Valid_constants(), x)

def openaiapi_ModerationMode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 108n, 111n, 99n, 107n], [115n, 99n, 111n, 114n, 101n]]

def openaiapi_ModerationMode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModerationMode_Valid_constants(), x)

def openaiapi_ModerationResultBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 111n, 100n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def openaiapi_ModerationResultBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ModerationResultBodyType_Valid_constants(), x)

def openaiapi_MoveParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 111n, 118n, 101n]]

def openaiapi_MoveParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_MoveParamType_Valid_constants(), x)

def openaiapi_NamespaceToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[110n, 97n, 109n, 101n, 115n, 112n, 97n, 99n, 101n]]

def openaiapi_NamespaceToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_NamespaceToolParamType_Valid_constants(), x)

def openaiapi_OutputMessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 115n, 115n, 105n, 115n, 116n, 97n, 110n, 116n]]

def openaiapi_OutputMessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_OutputMessageRole_Valid_constants(), x)

def openaiapi_OutputMessageStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_OutputMessageStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_OutputMessageStatus_Valid_constants(), x)

def openaiapi_OutputMessageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n]]

def openaiapi_OutputMessageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_OutputMessageType_Valid_constants(), x)

def openaiapi_OutputTextContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_OutputTextContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_OutputTextContentType_Valid_constants(), x)

def openaiapi_ProgramType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n]]

def openaiapi_ProgramType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramType_Valid_constants(), x)

def openaiapi_ProgramItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n]]

def openaiapi_ProgramItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramItemParamType_Valid_constants(), x)

def openaiapi_ProgramOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ProgramOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramOutputType_Valid_constants(), x)

def openaiapi_ProgramOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ProgramOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramOutputItemParamType_Valid_constants(), x)

def openaiapi_ProgramOutputItemStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ProgramOutputItemStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramOutputItemStatus_Valid_constants(), x)

def openaiapi_ProgramOutputStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ProgramOutputStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramOutputStatus_Valid_constants(), x)

def openaiapi_ProgramToolCallCallerType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n]]

def openaiapi_ProgramToolCallCallerType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramToolCallCallerType_Valid_constants(), x)

def openaiapi_ProgramToolCallCallerParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n]]

def openaiapi_ProgramToolCallCallerParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgramToolCallCallerParamType_Valid_constants(), x)

def openaiapi_ProgrammaticToolCallingParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n, 109n, 97n, 116n, 105n, 99n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 105n, 110n, 103n]]

def openaiapi_ProgrammaticToolCallingParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ProgrammaticToolCallingParamType_Valid_constants(), x)

def openaiapi_PromptCacheBreakpointConfigMode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 112n, 108n, 105n, 99n, 105n, 116n]]

def openaiapi_PromptCacheBreakpointConfigMode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_PromptCacheBreakpointConfigMode_Valid_constants(), x)

def openaiapi_PromptCacheBreakpointParamMode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 112n, 108n, 105n, 99n, 105n, 116n]]

def openaiapi_PromptCacheBreakpointParamMode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_PromptCacheBreakpointParamMode_Valid_constants(), x)

def openaiapi_PromptCacheModeEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 112n, 108n, 105n, 99n, 105n, 116n], [105n, 109n, 112n, 108n, 105n, 99n, 105n, 116n]]

def openaiapi_PromptCacheModeEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_PromptCacheModeEnum_Valid_constants(), x)

def openaiapi_PromptCacheTTLEnum_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[51n, 48n, 109n]]

def openaiapi_PromptCacheTTLEnum_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_PromptCacheTTLEnum_Valid_constants(), x)

def openaiapi_RankerVersionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [100n, 101n, 102n, 97n, 117n, 108n, 116n, 45n, 50n, 48n, 50n, 52n, 45n, 49n, 49n, 45n, 49n, 53n]]

def openaiapi_RankerVersionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_RankerVersionType_Valid_constants(), x)

def openaiapi_ReasoningContext_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 108n, 108n, 95n, 116n, 117n, 114n, 110n, 115n], [97n, 117n, 116n, 111n], [99n, 117n, 114n, 114n, 101n, 110n, 116n, 95n, 116n, 117n, 114n, 110n]]

def openaiapi_ReasoningContext_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningContext_Valid_constants(), x)

def openaiapi_ReasoningGenerateSummary_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [99n, 111n, 110n, 99n, 105n, 115n, 101n], [100n, 101n, 116n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ReasoningGenerateSummary_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningGenerateSummary_Valid_constants(), x)

def openaiapi_ReasoningSummary_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [99n, 111n, 110n, 99n, 105n, 115n, 101n], [100n, 101n, 116n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ReasoningSummary_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningSummary_Valid_constants(), x)

def openaiapi_ReasoningItemStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ReasoningItemStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningItemStatus_Valid_constants(), x)

def openaiapi_ReasoningItemType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n]]

def openaiapi_ReasoningItemType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningItemType_Valid_constants(), x)

def openaiapi_ReasoningModeEnum1_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n], [115n, 116n, 97n, 110n, 100n, 97n, 114n, 100n]]

def openaiapi_ReasoningModeEnum1_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningModeEnum1_Valid_constants(), x)

def openaiapi_ReasoningTextContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_ReasoningTextContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ReasoningTextContentType_Valid_constants(), x)

def openaiapi_RefusalContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 102n, 117n, 115n, 97n, 108n]]

def openaiapi_RefusalContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_RefusalContentType_Valid_constants(), x)

def openaiapi_ResponseIncompleteDetailsReason_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 102n, 105n, 108n, 116n, 101n, 114n], [109n, 97n, 120n, 95n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 111n, 107n, 101n, 110n, 115n]]

def openaiapi_ResponseIncompleteDetailsReason_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseIncompleteDetailsReason_Valid_constants(), x)

def openaiapi_ResponseObject_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n]]

def openaiapi_ResponseObject_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseObject_Valid_constants(), x)

def openaiapi_ResponsePromptCacheRetention_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 95n, 109n, 101n, 109n, 111n, 114n, 121n], [50n, 52n, 104n]]

def openaiapi_ResponsePromptCacheRetention_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponsePromptCacheRetention_Valid_constants(), x)

def openaiapi_ResponseStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 97n, 110n, 99n, 101n, 108n, 108n, 101n, 100n], [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n], [113n, 117n, 101n, 117n, 101n, 100n]]

def openaiapi_ResponseStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseStatus_Valid_constants(), x)

def openaiapi_ResponseTruncation_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [100n, 105n, 115n, 97n, 98n, 108n, 101n, 100n]]

def openaiapi_ResponseTruncation_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseTruncation_Valid_constants(), x)

def openaiapi_ResponseAudioDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 97n, 117n, 100n, 105n, 111n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseAudioDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseAudioDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseAudioDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 97n, 117n, 100n, 105n, 111n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseAudioDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseAudioDoneEventType_Valid_constants(), x)

def openaiapi_ResponseAudioTranscriptDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 97n, 117n, 100n, 105n, 111n, 46n, 116n, 114n, 97n, 110n, 115n, 99n, 114n, 105n, 112n, 116n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseAudioTranscriptDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseAudioTranscriptDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseAudioTranscriptDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 97n, 117n, 100n, 105n, 111n, 46n, 116n, 114n, 97n, 110n, 115n, 99n, 114n, 105n, 112n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseAudioTranscriptDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseAudioTranscriptDoneEventType_Valid_constants(), x)

def openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 99n, 111n, 100n, 101n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 95n, 99n, 111n, 100n, 101n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid_constants(), x)

def openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 105n, 110n, 103n]]

def openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid_constants(), x)

def openaiapi_ResponseCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseContentPartAddedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 112n, 97n, 114n, 116n, 46n, 97n, 100n, 100n, 101n, 100n]]

def openaiapi_ResponseContentPartAddedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseContentPartAddedEventType_Valid_constants(), x)

def openaiapi_ResponseContentPartDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 112n, 97n, 114n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseContentPartDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseContentPartDoneEventType_Valid_constants(), x)

def openaiapi_ResponseCreatedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 114n, 101n, 97n, 116n, 101n, 100n]]

def openaiapi_ResponseCreatedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCreatedEventType_Valid_constants(), x)

def openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 117n, 115n, 116n, 111n, 109n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 105n, 110n, 112n, 117n, 116n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseCustomToolCallInputDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 99n, 117n, 115n, 116n, 111n, 109n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 95n, 105n, 110n, 112n, 117n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseCustomToolCallInputDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseCustomToolCallInputDoneEventType_Valid_constants(), x)

def openaiapi_ResponseErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 105n, 111n, 95n, 112n, 111n, 108n, 105n, 99n, 121n], [100n, 97n, 116n, 97n, 95n, 114n, 101n, 115n, 105n, 100n, 101n, 110n, 99n, 121n, 95n, 109n, 105n, 115n, 109n, 97n, 116n, 99n, 104n], [101n, 109n, 112n, 116n, 121n, 95n, 105n, 109n, 97n, 103n, 101n, 95n, 102n, 105n, 108n, 101n], [102n, 97n, 105n, 108n, 101n, 100n, 95n, 116n, 111n, 95n, 100n, 111n, 119n, 110n, 108n, 111n, 97n, 100n, 95n, 105n, 109n, 97n, 103n, 101n], [105n, 109n, 97n, 103n, 101n, 95n, 99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 112n, 111n, 108n, 105n, 99n, 121n, 95n, 118n, 105n, 111n, 108n, 97n, 116n, 105n, 111n, 110n], [105n, 109n, 97n, 103n, 101n, 95n, 102n, 105n, 108n, 101n, 95n, 110n, 111n, 116n, 95n, 102n, 111n, 117n, 110n, 100n], [105n, 109n, 97n, 103n, 101n, 95n, 102n, 105n, 108n, 101n, 95n, 116n, 111n, 111n, 95n, 108n, 97n, 114n, 103n, 101n], [105n, 109n, 97n, 103n, 101n, 95n, 112n, 97n, 114n, 115n, 101n, 95n, 101n, 114n, 114n, 111n, 114n], [105n, 109n, 97n, 103n, 101n, 95n, 116n, 111n, 111n, 95n, 108n, 97n, 114n, 103n, 101n], [105n, 109n, 97n, 103n, 101n, 95n, 116n, 111n, 111n, 95n, 115n, 109n, 97n, 108n, 108n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 98n, 97n, 115n, 101n, 54n, 52n, 95n, 105n, 109n, 97n, 103n, 101n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 105n, 109n, 97n, 103n, 101n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 105n, 109n, 97n, 103n, 101n, 95n, 102n, 111n, 114n, 109n, 97n, 116n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 105n, 109n, 97n, 103n, 101n, 95n, 109n, 111n, 100n, 101n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 105n, 109n, 97n, 103n, 101n, 95n, 117n, 114n, 108n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 112n, 114n, 111n, 109n, 112n, 116n], [114n, 97n, 116n, 101n, 95n, 108n, 105n, 109n, 105n, 116n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [115n, 101n, 114n, 118n, 101n, 114n, 95n, 101n, 114n, 114n, 111n, 114n], [117n, 110n, 115n, 117n, 112n, 112n, 111n, 114n, 116n, 101n, 100n, 95n, 105n, 109n, 97n, 103n, 101n, 95n, 109n, 101n, 100n, 105n, 97n, 95n, 116n, 121n, 112n, 101n], [118n, 101n, 99n, 116n, 111n, 114n, 95n, 115n, 116n, 111n, 114n, 101n, 95n, 116n, 105n, 109n, 101n, 111n, 117n, 116n]]

def openaiapi_ResponseErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseErrorCode_Valid_constants(), x)

def openaiapi_ResponseErrorEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 114n, 114n, 111n, 114n]]

def openaiapi_ResponseErrorEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseErrorEventType_Valid_constants(), x)

def openaiapi_ResponseFailedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ResponseFailedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFailedEventType_Valid_constants(), x)

def openaiapi_ResponseFileSearchCallCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseFileSearchCallCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFileSearchCallCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseFileSearchCallInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseFileSearchCallInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFileSearchCallInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseFileSearchCallSearchingEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 115n, 101n, 97n, 114n, 99n, 104n, 105n, 110n, 103n]]

def openaiapi_ResponseFileSearchCallSearchingEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFileSearchCallSearchingEventType_Valid_constants(), x)

def openaiapi_ResponseFormatJsonObjectType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[106n, 115n, 111n, 110n, 95n, 111n, 98n, 106n, 101n, 99n, 116n]]

def openaiapi_ResponseFormatJsonObjectType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFormatJsonObjectType_Valid_constants(), x)

def openaiapi_ResponseFormatTextType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n]]

def openaiapi_ResponseFormatTextType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFormatTextType_Valid_constants(), x)

def openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 95n, 97n, 114n, 103n, 117n, 109n, 101n, 110n, 116n, 115n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 95n, 97n, 114n, 103n, 117n, 109n, 101n, 110n, 116n, 115n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid_constants(), x)

def openaiapi_ResponseImageGenCallCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseImageGenCallCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseImageGenCallCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseImageGenCallGeneratingEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 46n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 110n, 103n]]

def openaiapi_ResponseImageGenCallGeneratingEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseImageGenCallGeneratingEventType_Valid_constants(), x)

def openaiapi_ResponseImageGenCallInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseImageGenCallInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseImageGenCallInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseImageGenCallPartialImageEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n, 95n, 99n, 97n, 108n, 108n, 46n, 112n, 97n, 114n, 116n, 105n, 97n, 108n, 95n, 105n, 109n, 97n, 103n, 101n]]

def openaiapi_ResponseImageGenCallPartialImageEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseImageGenCallPartialImageEventType_Valid_constants(), x)

def openaiapi_ResponseInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseIncompleteEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ResponseIncompleteEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseIncompleteEventType_Valid_constants(), x)

def openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n, 95n, 97n, 114n, 103n, 117n, 109n, 101n, 110n, 116n, 115n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n, 95n, 97n, 114n, 103n, 117n, 109n, 101n, 110n, 116n, 115n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid_constants(), x)

def openaiapi_ResponseMCPCallCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseMCPCallCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPCallCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseMCPCallFailedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n, 46n, 102n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ResponseMCPCallFailedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPCallFailedEventType_Valid_constants(), x)

def openaiapi_ResponseMCPCallInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseMCPCallInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPCallInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseMCPListToolsCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 108n, 105n, 115n, 116n, 95n, 116n, 111n, 111n, 108n, 115n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseMCPListToolsCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPListToolsCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseMCPListToolsFailedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 108n, 105n, 115n, 116n, 95n, 116n, 111n, 111n, 108n, 115n, 46n, 102n, 97n, 105n, 108n, 101n, 100n]]

def openaiapi_ResponseMCPListToolsFailedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPListToolsFailedEventType_Valid_constants(), x)

def openaiapi_ResponseMCPListToolsInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 109n, 99n, 112n, 95n, 108n, 105n, 115n, 116n, 95n, 116n, 111n, 111n, 108n, 115n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseMCPListToolsInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseMCPListToolsInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseOutputItemAddedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 105n, 116n, 101n, 109n, 46n, 97n, 100n, 100n, 101n, 100n]]

def openaiapi_ResponseOutputItemAddedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseOutputItemAddedEventType_Valid_constants(), x)

def openaiapi_ResponseOutputItemDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 105n, 116n, 101n, 109n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseOutputItemDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseOutputItemDoneEventType_Valid_constants(), x)

def openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n, 46n, 97n, 110n, 110n, 111n, 116n, 97n, 116n, 105n, 111n, 110n, 46n, 97n, 100n, 100n, 101n, 100n]]

def openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid_constants(), x)

def openaiapi_ResponseQueuedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 113n, 117n, 101n, 117n, 101n, 100n]]

def openaiapi_ResponseQueuedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseQueuedEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 112n, 97n, 114n, 116n, 46n, 97n, 100n, 100n, 101n, 100n]]

def openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n]]

def openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 112n, 97n, 114n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningTextDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseReasoningTextDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningTextDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseReasoningTextDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseReasoningTextDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseReasoningTextDoneEventType_Valid_constants(), x)

def openaiapi_ResponseRefusalDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 102n, 117n, 115n, 97n, 108n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseRefusalDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseRefusalDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseRefusalDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 114n, 101n, 102n, 117n, 115n, 97n, 108n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseRefusalDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseRefusalDoneEventType_Valid_constants(), x)

def openaiapi_ResponseTextDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 101n, 108n, 116n, 97n]]

def openaiapi_ResponseTextDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseTextDeltaEventType_Valid_constants(), x)

def openaiapi_ResponseTextDoneEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 111n, 117n, 116n, 112n, 117n, 116n, 95n, 116n, 101n, 120n, 116n, 46n, 100n, 111n, 110n, 101n]]

def openaiapi_ResponseTextDoneEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseTextDoneEventType_Valid_constants(), x)

def openaiapi_ResponseWebSearchCallCompletedEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def openaiapi_ResponseWebSearchCallCompletedEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseWebSearchCallCompletedEventType_Valid_constants(), x)

def openaiapi_ResponseWebSearchCallInProgressEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n]]

def openaiapi_ResponseWebSearchCallInProgressEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseWebSearchCallInProgressEventType_Valid_constants(), x)

def openaiapi_ResponseWebSearchCallSearchingEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 115n, 112n, 111n, 110n, 115n, 101n, 46n, 119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n, 46n, 115n, 101n, 97n, 114n, 99n, 104n, 105n, 110n, 103n]]

def openaiapi_ResponseWebSearchCallSearchingEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ResponseWebSearchCallSearchingEventType_Valid_constants(), x)

def openaiapi_ScreenshotParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 99n, 114n, 101n, 101n, 110n, 115n, 104n, 111n, 116n]]

def openaiapi_ScreenshotParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ScreenshotParamType_Valid_constants(), x)

def openaiapi_ScrollParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 99n, 114n, 111n, 108n, 108n]]

def openaiapi_ScrollParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ScrollParamType_Valid_constants(), x)

def openaiapi_SearchContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n], [116n, 101n, 120n, 116n]]

def openaiapi_SearchContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SearchContentType_Valid_constants(), x)

def openaiapi_SearchContextSize_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[104n, 105n, 103n, 104n], [108n, 111n, 119n], [109n, 101n, 100n, 105n, 117n, 109n]]

def openaiapi_SearchContextSize_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SearchContextSize_Valid_constants(), x)

def openaiapi_SkillReferenceParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 107n, 105n, 108n, 108n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def openaiapi_SkillReferenceParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SkillReferenceParamType_Valid_constants(), x)

def openaiapi_SpecificApplyPatchParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 121n, 95n, 112n, 97n, 116n, 99n, 104n]]

def openaiapi_SpecificApplyPatchParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SpecificApplyPatchParamType_Valid_constants(), x)

def openaiapi_SpecificFunctionShellParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 104n, 101n, 108n, 108n]]

def openaiapi_SpecificFunctionShellParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SpecificFunctionShellParamType_Valid_constants(), x)

def openaiapi_SpecificProgrammaticToolCallingParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 114n, 111n, 103n, 114n, 97n, 109n, 109n, 97n, 116n, 105n, 99n, 95n, 116n, 111n, 111n, 108n, 95n, 99n, 97n, 108n, 108n, 105n, 110n, 103n]]

def openaiapi_SpecificProgrammaticToolCallingParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SpecificProgrammaticToolCallingParamType_Valid_constants(), x)

def openaiapi_SummaryTextContentType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 117n, 109n, 109n, 97n, 114n, 121n, 95n, 116n, 101n, 120n, 116n]]

def openaiapi_SummaryTextContentType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_SummaryTextContentType_Valid_constants(), x)

def openaiapi_TextResponseFormatJsonSchemaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[106n, 115n, 111n, 110n, 95n, 115n, 99n, 104n, 101n, 109n, 97n]]

def openaiapi_TextResponseFormatJsonSchemaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_TextResponseFormatJsonSchemaType_Valid_constants(), x)

def openaiapi_ToolChoiceAllowedMode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [114n, 101n, 113n, 117n, 105n, 114n, 101n, 100n]]

def openaiapi_ToolChoiceAllowedMode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceAllowedMode_Valid_constants(), x)

def openaiapi_ToolChoiceAllowedType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 108n, 108n, 111n, 119n, 101n, 100n, 95n, 116n, 111n, 111n, 108n, 115n]]

def openaiapi_ToolChoiceAllowedType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceAllowedType_Valid_constants(), x)

def openaiapi_ToolChoiceCustomType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n]]

def openaiapi_ToolChoiceCustomType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceCustomType_Valid_constants(), x)

def openaiapi_ToolChoiceFunctionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 117n, 110n, 99n, 116n, 105n, 111n, 110n]]

def openaiapi_ToolChoiceFunctionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceFunctionType_Valid_constants(), x)

def openaiapi_ToolChoiceMCPType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 99n, 112n]]

def openaiapi_ToolChoiceMCPType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceMCPType_Valid_constants(), x)

def openaiapi_ToolChoiceOptions_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [110n, 111n, 110n, 101n], [114n, 101n, 113n, 117n, 105n, 114n, 101n, 100n]]

def openaiapi_ToolChoiceOptions_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceOptions_Valid_constants(), x)

def openaiapi_ToolChoiceTypesType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 105n, 110n, 116n, 101n, 114n, 112n, 114n, 101n, 116n, 101n, 114n], [99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n], [99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 117n, 115n, 101n], [99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 117n, 115n, 101n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [102n, 105n, 108n, 101n, 95n, 115n, 101n, 97n, 114n, 99n, 104n], [105n, 109n, 97n, 103n, 101n, 95n, 103n, 101n, 110n, 101n, 114n, 97n, 116n, 105n, 111n, 110n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 95n, 50n, 48n, 50n, 53n, 95n, 48n, 51n, 95n, 49n, 49n]]

def openaiapi_ToolChoiceTypesType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolChoiceTypesType_Valid_constants(), x)

def openaiapi_ToolSearchCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ToolSearchCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchCallType_Valid_constants(), x)

def openaiapi_ToolSearchCallItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_ToolSearchCallItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchCallItemParamType_Valid_constants(), x)

def openaiapi_ToolSearchExecutionType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 108n, 105n, 101n, 110n, 116n], [115n, 101n, 114n, 118n, 101n, 114n]]

def openaiapi_ToolSearchExecutionType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchExecutionType_Valid_constants(), x)

def openaiapi_ToolSearchOutputType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ToolSearchOutputType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchOutputType_Valid_constants(), x)

def openaiapi_ToolSearchOutputItemParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def openaiapi_ToolSearchOutputItemParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchOutputItemParamType_Valid_constants(), x)

def openaiapi_ToolSearchToolParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def openaiapi_ToolSearchToolParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_ToolSearchToolParamType_Valid_constants(), x)

def openaiapi_TypeParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 121n, 112n, 101n]]

def openaiapi_TypeParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_TypeParamType_Valid_constants(), x)

def openaiapi_UrlCitationBodyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 114n, 108n, 95n, 99n, 105n, 116n, 97n, 116n, 105n, 111n, 110n]]

def openaiapi_UrlCitationBodyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_UrlCitationBodyType_Valid_constants(), x)

def openaiapi_WaitParamType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 97n, 105n, 116n]]

def openaiapi_WaitParamType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WaitParamType_Valid_constants(), x)

def openaiapi_WebSearchActionFindType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 110n, 100n, 95n, 105n, 110n, 95n, 112n, 97n, 103n, 101n]]

def openaiapi_WebSearchActionFindType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchActionFindType_Valid_constants(), x)

def openaiapi_WebSearchActionOpenPageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 112n, 101n, 110n, 95n, 112n, 97n, 103n, 101n]]

def openaiapi_WebSearchActionOpenPageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchActionOpenPageType_Valid_constants(), x)

def openaiapi_WebSearchActionSearchSourcesType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 114n, 108n]]

def openaiapi_WebSearchActionSearchSourcesType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchActionSearchSourcesType_Valid_constants(), x)

def openaiapi_WebSearchActionSearchType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 97n, 114n, 99n, 104n]]

def openaiapi_WebSearchActionSearchType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchActionSearchType_Valid_constants(), x)

def openaiapi_WebSearchApproximateLocationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 114n, 111n, 120n, 105n, 109n, 97n, 116n, 101n]]

def openaiapi_WebSearchApproximateLocationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchApproximateLocationType_Valid_constants(), x)

def openaiapi_WebSearchPreviewToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 112n, 114n, 101n, 118n, 105n, 101n, 119n, 95n, 50n, 48n, 50n, 53n, 95n, 48n, 51n, 95n, 49n, 49n]]

def openaiapi_WebSearchPreviewToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchPreviewToolType_Valid_constants(), x)

def openaiapi_WebSearchToolSearchContextSize_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[104n, 105n, 103n, 104n], [108n, 111n, 119n], [109n, 101n, 100n, 105n, 117n, 109n]]

def openaiapi_WebSearchToolSearchContextSize_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchToolSearchContextSize_Valid_constants(), x)

def openaiapi_WebSearchToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 50n, 48n, 50n, 53n, 95n, 48n, 56n, 95n, 50n, 54n]]

def openaiapi_WebSearchToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchToolType_Valid_constants(), x)

def openaiapi_WebSearchToolCallStatus_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n], [102n, 97n, 105n, 108n, 101n, 100n], [105n, 110n, 95n, 112n, 114n, 111n, 103n, 114n, 101n, 115n, 115n], [115n, 101n, 97n, 114n, 99n, 104n, 105n, 110n, 103n]]

def openaiapi_WebSearchToolCallStatus_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchToolCallStatus_Valid_constants(), x)

def openaiapi_WebSearchToolCallType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 99n, 97n, 108n, 108n]]

def openaiapi_WebSearchToolCallType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(openaiapi_WebSearchToolCallType_Valid_constants(), x)

def anthropicapi_APIErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 105n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_APIErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_APIErrorType_Valid_constants(), x)

def anthropicapi_AllowedCaller_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 53n, 48n, 56n, 50n, 53n], [99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 54n, 48n, 49n, 50n, 48n], [99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 54n, 48n, 53n, 50n, 49n], [100n, 105n, 114n, 101n, 99n, 116n]]

def anthropicapi_AllowedCaller_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_AllowedCaller_Valid_constants(), x)

def anthropicapi_AuthenticationErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 104n, 101n, 110n, 116n, 105n, 99n, 97n, 116n, 105n, 111n, 110n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_AuthenticationErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_AuthenticationErrorType_Valid_constants(), x)

def anthropicapi_Base64ImageSourceMediaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n, 47n, 103n, 105n, 102n], [105n, 109n, 97n, 103n, 101n, 47n, 106n, 112n, 101n, 103n], [105n, 109n, 97n, 103n, 101n, 47n, 112n, 110n, 103n], [105n, 109n, 97n, 103n, 101n, 47n, 119n, 101n, 98n, 112n]]

def anthropicapi_Base64ImageSourceMediaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_Base64ImageSourceMediaType_Valid_constants(), x)

def anthropicapi_Base64ImageSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 101n, 54n, 52n]]

def anthropicapi_Base64ImageSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_Base64ImageSourceType_Valid_constants(), x)

def anthropicapi_Base64PDFSourceMediaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 108n, 105n, 99n, 97n, 116n, 105n, 111n, 110n, 47n, 112n, 100n, 102n]]

def anthropicapi_Base64PDFSourceMediaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_Base64PDFSourceMediaType_Valid_constants(), x)

def anthropicapi_Base64PDFSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 101n, 54n, 52n]]

def anthropicapi_Base64PDFSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_Base64PDFSourceType_Valid_constants(), x)

def anthropicapi_BashCodeExecutionToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 105n, 109n, 101n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [111n, 117n, 116n, 112n, 117n, 116n, 95n, 102n, 105n, 108n, 101n, 95n, 116n, 111n, 111n, 95n, 108n, 97n, 114n, 103n, 101n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n]]

def anthropicapi_BashCodeExecutionToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BashCodeExecutionToolResultErrorCode_Valid_constants(), x)

def anthropicapi_BashTool20250124Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n]]

def anthropicapi_BashTool20250124Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BashTool20250124Name_Valid_constants(), x)

def anthropicapi_BashTool20250124Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 50n, 48n, 50n, 53n, 48n, 49n, 50n, 52n]]

def anthropicapi_BashTool20250124Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BashTool20250124Type_Valid_constants(), x)

def anthropicapi_BillingErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 105n, 108n, 108n, 105n, 110n, 103n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_BillingErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BillingErrorType_Valid_constants(), x)

def anthropicapi_BrowserStateChangeDownloadCompletedType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 119n, 110n, 108n, 111n, 97n, 100n, 95n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]]

def anthropicapi_BrowserStateChangeDownloadCompletedType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BrowserStateChangeDownloadCompletedType_Valid_constants(), x)

def anthropicapi_BrowserStateChangeDownloadFailedType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 119n, 110n, 108n, 111n, 97n, 100n, 95n, 102n, 97n, 105n, 108n, 101n, 100n]]

def anthropicapi_BrowserStateChangeDownloadFailedType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BrowserStateChangeDownloadFailedType_Valid_constants(), x)

def anthropicapi_BrowserStateChangeDownloadStartedType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 119n, 110n, 108n, 111n, 97n, 100n, 95n, 115n, 116n, 97n, 114n, 116n, 101n, 100n]]

def anthropicapi_BrowserStateChangeDownloadStartedType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BrowserStateChangeDownloadStartedType_Valid_constants(), x)

def anthropicapi_BrowserStateChangeTabOpenedType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 97n, 98n, 95n, 111n, 112n, 101n, 110n, 101n, 100n]]

def anthropicapi_BrowserStateChangeTabOpenedType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BrowserStateChangeTabOpenedType_Valid_constants(), x)

def anthropicapi_BrowserToolset20260801Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 114n, 111n, 119n, 115n, 101n, 114n, 95n, 116n, 111n, 111n, 108n, 115n, 101n, 116n, 95n, 50n, 48n, 50n, 54n, 48n, 56n, 48n, 49n]]

def anthropicapi_BrowserToolset20260801Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_BrowserToolset20260801Type_Valid_constants(), x)

def anthropicapi_CacheControlEphemeralTtl_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[49n, 104n], [53n, 109n]]

def anthropicapi_CacheControlEphemeralTtl_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CacheControlEphemeralTtl_Valid_constants(), x)

def anthropicapi_CacheControlEphemeralType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 112n, 104n, 101n, 109n, 101n, 114n, 97n, 108n]]

def anthropicapi_CacheControlEphemeralType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CacheControlEphemeralType_Valid_constants(), x)

def anthropicapi_CitationsDeltaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 105n, 116n, 97n, 116n, 105n, 111n, 110n, 115n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_CitationsDeltaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CitationsDeltaType_Valid_constants(), x)

def anthropicapi_CodeExecutionToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 105n, 109n, 101n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n]]

def anthropicapi_CodeExecutionToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionToolResultErrorCode_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20250522Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n]]

def anthropicapi_CodeExecutionTool20250522Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20250522Name_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20250522Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 53n, 48n, 53n, 50n, 50n]]

def anthropicapi_CodeExecutionTool20250522Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20250522Type_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20250825Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n]]

def anthropicapi_CodeExecutionTool20250825Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20250825Name_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20250825Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 53n, 48n, 56n, 50n, 53n]]

def anthropicapi_CodeExecutionTool20250825Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20250825Type_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20260120Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n]]

def anthropicapi_CodeExecutionTool20260120Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20260120Name_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20260120Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 54n, 48n, 49n, 50n, 48n]]

def anthropicapi_CodeExecutionTool20260120Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20260120Type_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20260521Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n]]

def anthropicapi_CodeExecutionTool20260521Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20260521Name_Valid_constants(), x)

def anthropicapi_CodeExecutionTool20260521Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 54n, 48n, 53n, 50n, 49n]]

def anthropicapi_CodeExecutionTool20260521Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CodeExecutionTool20260521Type_Valid_constants(), x)

def anthropicapi_ComputerToolset20260801Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 109n, 112n, 117n, 116n, 101n, 114n, 95n, 116n, 111n, 111n, 108n, 115n, 101n, 116n, 95n, 50n, 48n, 50n, 54n, 48n, 56n, 48n, 49n]]

def anthropicapi_ComputerToolset20260801Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ComputerToolset20260801Type_Valid_constants(), x)

def anthropicapi_ContainerSkillType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 110n, 116n, 104n, 114n, 111n, 112n, 105n, 99n], [99n, 117n, 115n, 116n, 111n, 109n]]

def anthropicapi_ContainerSkillType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ContainerSkillType_Valid_constants(), x)

def anthropicapi_ContentBlockDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 98n, 108n, 111n, 99n, 107n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_ContentBlockDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ContentBlockDeltaEventType_Valid_constants(), x)

def anthropicapi_ContentBlockSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n]]

def anthropicapi_ContentBlockSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ContentBlockSourceType_Valid_constants(), x)

def anthropicapi_ContentBlockStartEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 98n, 108n, 111n, 99n, 107n, 95n, 115n, 116n, 97n, 114n, 116n]]

def anthropicapi_ContentBlockStartEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ContentBlockStartEventType_Valid_constants(), x)

def anthropicapi_ContentBlockStopEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 98n, 108n, 111n, 99n, 107n, 95n, 115n, 116n, 111n, 112n]]

def anthropicapi_ContentBlockStopEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ContentBlockStopEventType_Valid_constants(), x)

def anthropicapi_CreateMessageParamsServiceTier_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n], [115n, 116n, 97n, 110n, 100n, 97n, 114n, 100n, 95n, 111n, 110n, 108n, 121n]]

def anthropicapi_CreateMessageParamsServiceTier_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_CreateMessageParamsServiceTier_Valid_constants(), x)

def anthropicapi_DirectCallerType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 114n, 101n, 99n, 116n]]

def anthropicapi_DirectCallerType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_DirectCallerType_Valid_constants(), x)

def anthropicapi_EffortLevel_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[104n, 105n, 103n, 104n], [108n, 111n, 119n], [109n, 97n, 120n], [109n, 101n, 100n, 105n, 117n, 109n], [120n, 104n, 105n, 103n, 104n]]

def anthropicapi_EffortLevel_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_EffortLevel_Valid_constants(), x)

def anthropicapi_ErrorResponseType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ErrorResponseType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ErrorResponseType_Valid_constants(), x)

def anthropicapi_FileDocumentSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n]]

def anthropicapi_FileDocumentSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_FileDocumentSourceType_Valid_constants(), x)

def anthropicapi_FileImageSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[102n, 105n, 108n, 101n]]

def anthropicapi_FileImageSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_FileImageSourceType_Valid_constants(), x)

def anthropicapi_GatewayTimeoutErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 105n, 109n, 101n, 111n, 117n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_GatewayTimeoutErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_GatewayTimeoutErrorType_Valid_constants(), x)

def anthropicapi_InputJsonContentBlockDeltaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 112n, 117n, 116n, 95n, 106n, 115n, 111n, 110n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_InputJsonContentBlockDeltaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_InputJsonContentBlockDeltaType_Valid_constants(), x)

def anthropicapi_InputMessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 115n, 115n, 105n, 115n, 116n, 97n, 110n, 116n], [115n, 121n, 115n, 116n, 101n, 109n], [117n, 115n, 101n, 114n]]

def anthropicapi_InputMessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_InputMessageRole_Valid_constants(), x)

def anthropicapi_InputSchemaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 98n, 106n, 101n, 99n, 116n]]

def anthropicapi_InputSchemaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_InputSchemaType_Valid_constants(), x)

def anthropicapi_InvalidRequestErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_InvalidRequestErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_InvalidRequestErrorType_Valid_constants(), x)

def anthropicapi_JsonOutputFormatType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[106n, 115n, 111n, 110n, 95n, 115n, 99n, 104n, 101n, 109n, 97n]]

def anthropicapi_JsonOutputFormatType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_JsonOutputFormatType_Valid_constants(), x)

def anthropicapi_MemoryTool20250818Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 109n, 111n, 114n, 121n]]

def anthropicapi_MemoryTool20250818Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MemoryTool20250818Name_Valid_constants(), x)

def anthropicapi_MemoryTool20250818Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 109n, 111n, 114n, 121n, 95n, 50n, 48n, 50n, 53n, 48n, 56n, 49n, 56n]]

def anthropicapi_MemoryTool20250818Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MemoryTool20250818Type_Valid_constants(), x)

def anthropicapi_MessageRole_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 115n, 115n, 105n, 115n, 116n, 97n, 110n, 116n]]

def anthropicapi_MessageRole_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MessageRole_Valid_constants(), x)

def anthropicapi_MessageType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n]]

def anthropicapi_MessageType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MessageType_Valid_constants(), x)

def anthropicapi_MessageDeltaEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_MessageDeltaEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MessageDeltaEventType_Valid_constants(), x)

def anthropicapi_MessageStartEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n, 95n, 115n, 116n, 97n, 114n, 116n]]

def anthropicapi_MessageStartEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MessageStartEventType_Valid_constants(), x)

def anthropicapi_MessageStopEventType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[109n, 101n, 115n, 115n, 97n, 103n, 101n, 95n, 115n, 116n, 111n, 112n]]

def anthropicapi_MessageStopEventType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_MessageStopEventType_Valid_constants(), x)

def anthropicapi_NotFoundErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[110n, 111n, 116n, 95n, 102n, 111n, 117n, 110n, 100n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_NotFoundErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_NotFoundErrorType_Valid_constants(), x)

def anthropicapi_OverloadedErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 118n, 101n, 114n, 108n, 111n, 97n, 100n, 101n, 100n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_OverloadedErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_OverloadedErrorType_Valid_constants(), x)

def anthropicapi_PermissionErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 101n, 114n, 109n, 105n, 115n, 115n, 105n, 111n, 110n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_PermissionErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_PermissionErrorType_Valid_constants(), x)

def anthropicapi_PlainTextSourceMediaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 47n, 112n, 108n, 97n, 105n, 110n]]

def anthropicapi_PlainTextSourceMediaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_PlainTextSourceMediaType_Valid_constants(), x)

def anthropicapi_PlainTextSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n]]

def anthropicapi_PlainTextSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_PlainTextSourceType_Valid_constants(), x)

def anthropicapi_RateLimitErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 97n, 116n, 101n, 95n, 108n, 105n, 109n, 105n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RateLimitErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RateLimitErrorType_Valid_constants(), x)

def anthropicapi_RefusalCategory_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 105n, 111n], [99n, 121n, 98n, 101n, 114n], [102n, 114n, 111n, 110n, 116n, 105n, 101n, 114n, 95n, 108n, 108n, 109n], [103n, 101n, 110n, 101n, 114n, 97n, 108n, 95n, 104n, 97n, 114n, 109n, 115n], [114n, 101n, 97n, 115n, 111n, 110n, 105n, 110n, 103n, 95n, 101n, 120n, 116n, 114n, 97n, 99n, 116n, 105n, 111n, 110n]]

def anthropicapi_RefusalCategory_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RefusalCategory_Valid_constants(), x)

def anthropicapi_RefusalStopDetailsType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 102n, 117n, 115n, 97n, 108n]]

def anthropicapi_RefusalStopDetailsType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RefusalStopDetailsType_Valid_constants(), x)

def anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid_constants(), x)

def anthropicapi_RequestBashCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestBashCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestBashCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_RequestBrowserStateBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 114n, 111n, 119n, 115n, 101n, 114n, 95n, 115n, 116n, 97n, 116n, 101n]]

def anthropicapi_RequestBrowserStateBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestBrowserStateBlockType_Valid_constants(), x)

def anthropicapi_RequestCharLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 104n, 97n, 114n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_RequestCharLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestCharLocationCitationType_Valid_constants(), x)

def anthropicapi_RequestCodeExecutionOutputBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def anthropicapi_RequestCodeExecutionOutputBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestCodeExecutionOutputBlockType_Valid_constants(), x)

def anthropicapi_RequestCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_RequestCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_RequestContainerUploadBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 117n, 112n, 108n, 111n, 97n, 100n]]

def anthropicapi_RequestContainerUploadBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestContainerUploadBlockType_Valid_constants(), x)

def anthropicapi_RequestContentBlockLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 98n, 108n, 111n, 99n, 107n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_RequestContentBlockLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestContentBlockLocationCitationType_Valid_constants(), x)

def anthropicapi_RequestDocumentBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 99n, 117n, 109n, 101n, 110n, 116n]]

def anthropicapi_RequestDocumentBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestDocumentBlockType_Valid_constants(), x)

def anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 110n, 99n, 114n, 121n, 112n, 116n, 101n, 100n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_RequestImageBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n]]

def anthropicapi_RequestImageBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestImageBlockType_Valid_constants(), x)

def anthropicapi_RequestImageTransformationsOversizedImage_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 119n, 110n, 115n, 105n, 122n, 101n], [101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestImageTransformationsOversizedImage_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestImageTransformationsOversizedImage_Valid_constants(), x)

def anthropicapi_RequestPageLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 97n, 103n, 101n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_RequestPageLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestPageLocationCitationType_Valid_constants(), x)

def anthropicapi_RequestRedactedThinkingBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 100n, 97n, 99n, 116n, 101n, 100n, 95n, 116n, 104n, 105n, 110n, 107n, 105n, 110n, 103n]]

def anthropicapi_RequestRedactedThinkingBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestRedactedThinkingBlockType_Valid_constants(), x)

def anthropicapi_RequestSearchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestSearchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestSearchResultBlockType_Valid_constants(), x)

def anthropicapi_RequestSearchResultLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_RequestSearchResultLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestSearchResultLocationCitationType_Valid_constants(), x)

def anthropicapi_RequestServerToolUseBlockName_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 98n, 109n, 50n, 53n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 103n, 101n, 120n], [119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def anthropicapi_RequestServerToolUseBlockName_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestServerToolUseBlockName_Valid_constants(), x)

def anthropicapi_RequestServerToolUseBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 114n, 118n, 101n, 114n, 95n, 116n, 111n, 111n, 108n, 95n, 117n, 115n, 101n]]

def anthropicapi_RequestServerToolUseBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestServerToolUseBlockType_Valid_constants(), x)

def anthropicapi_RequestTextBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n]]

def anthropicapi_RequestTextBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextBlockType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 99n, 114n, 101n, 97n, 116n, 101n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 115n, 116n, 114n, 95n, 114n, 101n, 112n, 108n, 97n, 99n, 101n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n], [112n, 100n, 102n], [116n, 101n, 120n, 116n]]

def anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid_constants(), x)

def anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 118n, 105n, 101n, 119n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid_constants(), x)

def anthropicapi_RequestThinkingBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 104n, 105n, 110n, 107n, 105n, 110n, 103n]]

def anthropicapi_RequestThinkingBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestThinkingBlockType_Valid_constants(), x)

def anthropicapi_RequestToolReferenceBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def anthropicapi_RequestToolReferenceBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolReferenceBlockType_Valid_constants(), x)

def anthropicapi_RequestToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestToolSearchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestToolSearchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolSearchToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestToolSearchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestToolSearchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolSearchToolResultErrorType_Valid_constants(), x)

def anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid_constants(), x)

def anthropicapi_RequestToolUseBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 117n, 115n, 101n]]

def anthropicapi_RequestToolUseBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestToolUseBlockType_Valid_constants(), x)

def anthropicapi_RequestWebFetchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestWebFetchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebFetchResultBlockType_Valid_constants(), x)

def anthropicapi_RequestWebFetchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestWebFetchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebFetchToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestWebFetchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestWebFetchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebFetchToolResultErrorType_Valid_constants(), x)

def anthropicapi_RequestWebSearchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestWebSearchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebSearchResultBlockType_Valid_constants(), x)

def anthropicapi_RequestWebSearchResultLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_RequestWebSearchResultLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebSearchResultLocationCitationType_Valid_constants(), x)

def anthropicapi_RequestWebSearchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_RequestWebSearchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebSearchToolResultBlockType_Valid_constants(), x)

def anthropicapi_RequestWebSearchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_RequestWebSearchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_RequestWebSearchToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid_constants(), x)

def anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseCharLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 104n, 97n, 114n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_ResponseCharLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseCharLocationCitationType_Valid_constants(), x)

def anthropicapi_ResponseCodeExecutionOutputBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 111n, 117n, 116n, 112n, 117n, 116n]]

def anthropicapi_ResponseCodeExecutionOutputBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseCodeExecutionOutputBlockType_Valid_constants(), x)

def anthropicapi_ResponseCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseContainerUploadBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 97n, 105n, 110n, 101n, 114n, 95n, 117n, 112n, 108n, 111n, 97n, 100n]]

def anthropicapi_ResponseContainerUploadBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseContainerUploadBlockType_Valid_constants(), x)

def anthropicapi_ResponseContentBlockLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 98n, 108n, 111n, 99n, 107n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_ResponseContentBlockLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseContentBlockLocationCitationType_Valid_constants(), x)

def anthropicapi_ResponseDocumentBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 111n, 99n, 117n, 109n, 101n, 110n, 116n]]

def anthropicapi_ResponseDocumentBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseDocumentBlockType_Valid_constants(), x)

def anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 110n, 99n, 114n, 121n, 112n, 116n, 101n, 100n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid_constants(), x)

def anthropicapi_ResponsePageLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[112n, 97n, 103n, 101n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_ResponsePageLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponsePageLocationCitationType_Valid_constants(), x)

def anthropicapi_ResponseRedactedThinkingBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[114n, 101n, 100n, 97n, 99n, 116n, 101n, 100n, 95n, 116n, 104n, 105n, 110n, 107n, 105n, 110n, 103n]]

def anthropicapi_ResponseRedactedThinkingBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseRedactedThinkingBlockType_Valid_constants(), x)

def anthropicapi_ResponseSearchResultLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_ResponseSearchResultLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseSearchResultLocationCitationType_Valid_constants(), x)

def anthropicapi_ResponseServerToolUseBlockName_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 115n, 104n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 98n, 109n, 50n, 53n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 103n, 101n, 120n], [119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n], [119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def anthropicapi_ResponseServerToolUseBlockName_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseServerToolUseBlockName_Valid_constants(), x)

def anthropicapi_ResponseServerToolUseBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 101n, 114n, 118n, 101n, 114n, 95n, 116n, 111n, 111n, 108n, 95n, 117n, 115n, 101n]]

def anthropicapi_ResponseServerToolUseBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseServerToolUseBlockType_Valid_constants(), x)

def anthropicapi_ResponseTextBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n]]

def anthropicapi_ResponseTextBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextBlockType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 99n, 114n, 101n, 97n, 116n, 101n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 115n, 116n, 114n, 95n, 114n, 101n, 112n, 108n, 97n, 99n, 101n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 109n, 97n, 103n, 101n], [112n, 100n, 102n], [116n, 101n, 120n, 116n]]

def anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid_constants(), x)

def anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 118n, 105n, 101n, 119n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseThinkingBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 104n, 105n, 110n, 107n, 105n, 110n, 103n]]

def anthropicapi_ResponseThinkingBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseThinkingBlockType_Valid_constants(), x)

def anthropicapi_ResponseToolReferenceBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def anthropicapi_ResponseToolReferenceBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseToolReferenceBlockType_Valid_constants(), x)

def anthropicapi_ResponseToolSearchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseToolSearchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseToolSearchToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseToolSearchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseToolSearchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseToolSearchToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseToolUseBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 117n, 115n, 101n]]

def anthropicapi_ResponseToolUseBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseToolUseBlockType_Valid_constants(), x)

def anthropicapi_ResponseWebFetchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseWebFetchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebFetchResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseWebFetchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseWebFetchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebFetchToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseWebFetchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseWebFetchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebFetchToolResultErrorType_Valid_constants(), x)

def anthropicapi_ResponseWebSearchResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseWebSearchResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebSearchResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseWebSearchResultLocationCitationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 108n, 111n, 99n, 97n, 116n, 105n, 111n, 110n]]

def anthropicapi_ResponseWebSearchResultLocationCitationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebSearchResultLocationCitationType_Valid_constants(), x)

def anthropicapi_ResponseWebSearchToolResultBlockType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n]]

def anthropicapi_ResponseWebSearchToolResultBlockType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebSearchToolResultBlockType_Valid_constants(), x)

def anthropicapi_ResponseWebSearchToolResultErrorType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 115n, 117n, 108n, 116n, 95n, 101n, 114n, 114n, 111n, 114n]]

def anthropicapi_ResponseWebSearchToolResultErrorType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ResponseWebSearchToolResultErrorType_Valid_constants(), x)

def anthropicapi_ServerToolCallerType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 53n, 48n, 56n, 50n, 53n]]

def anthropicapi_ServerToolCallerType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ServerToolCallerType_Valid_constants(), x)

def anthropicapi_ServerToolCaller20260120Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 100n, 101n, 95n, 101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 50n, 48n, 50n, 54n, 48n, 49n, 50n, 48n]]

def anthropicapi_ServerToolCaller20260120Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ServerToolCaller20260120Type_Valid_constants(), x)

def anthropicapi_SignatureContentBlockDeltaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 105n, 103n, 110n, 97n, 116n, 117n, 114n, 101n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_SignatureContentBlockDeltaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_SignatureContentBlockDeltaType_Valid_constants(), x)

def anthropicapi_SkillParamsType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 110n, 116n, 104n, 114n, 111n, 112n, 105n, 99n], [99n, 117n, 115n, 116n, 111n, 109n]]

def anthropicapi_SkillParamsType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_SkillParamsType_Valid_constants(), x)

def anthropicapi_StopReason_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 110n, 100n, 95n, 116n, 117n, 114n, 110n], [109n, 97n, 120n, 95n, 116n, 111n, 107n, 101n, 110n, 115n], [109n, 111n, 100n, 101n, 108n, 95n, 99n, 111n, 110n, 116n, 101n, 120n, 116n, 95n, 119n, 105n, 110n, 100n, 111n, 119n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [112n, 97n, 117n, 115n, 101n, 95n, 116n, 117n, 114n, 110n], [114n, 101n, 102n, 117n, 115n, 97n, 108n], [115n, 116n, 111n, 112n, 95n, 115n, 101n, 113n, 117n, 101n, 110n, 99n, 101n], [116n, 111n, 111n, 108n, 95n, 117n, 115n, 101n]]

def anthropicapi_StopReason_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_StopReason_Valid_constants(), x)

def anthropicapi_TextContentBlockDeltaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_TextContentBlockDeltaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextContentBlockDeltaType_Valid_constants(), x)

def anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 105n, 109n, 101n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [102n, 105n, 108n, 101n, 95n, 110n, 111n, 116n, 95n, 102n, 111n, 117n, 110n, 100n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n]]

def anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid_constants(), x)

def anthropicapi_TextEditor20250124Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 116n, 114n, 95n, 114n, 101n, 112n, 108n, 97n, 99n, 101n, 95n, 101n, 100n, 105n, 116n, 111n, 114n]]

def anthropicapi_TextEditor20250124Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250124Name_Valid_constants(), x)

def anthropicapi_TextEditor20250124Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 50n, 48n, 50n, 53n, 48n, 49n, 50n, 52n]]

def anthropicapi_TextEditor20250124Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250124Type_Valid_constants(), x)

def anthropicapi_TextEditor20250429Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 116n, 114n, 95n, 114n, 101n, 112n, 108n, 97n, 99n, 101n, 95n, 98n, 97n, 115n, 101n, 100n, 95n, 101n, 100n, 105n, 116n, 95n, 116n, 111n, 111n, 108n]]

def anthropicapi_TextEditor20250429Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250429Name_Valid_constants(), x)

def anthropicapi_TextEditor20250429Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 50n, 48n, 50n, 53n, 48n, 52n, 50n, 57n]]

def anthropicapi_TextEditor20250429Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250429Type_Valid_constants(), x)

def anthropicapi_TextEditor20250728Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[115n, 116n, 114n, 95n, 114n, 101n, 112n, 108n, 97n, 99n, 101n, 95n, 98n, 97n, 115n, 101n, 100n, 95n, 101n, 100n, 105n, 116n, 95n, 116n, 111n, 111n, 108n]]

def anthropicapi_TextEditor20250728Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250728Name_Valid_constants(), x)

def anthropicapi_TextEditor20250728Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 101n, 120n, 116n, 95n, 101n, 100n, 105n, 116n, 111n, 114n, 95n, 50n, 48n, 50n, 53n, 48n, 55n, 50n, 56n]]

def anthropicapi_TextEditor20250728Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_TextEditor20250728Type_Valid_constants(), x)

def anthropicapi_ThinkingConfigAdaptiveType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 100n, 97n, 112n, 116n, 105n, 118n, 101n]]

def anthropicapi_ThinkingConfigAdaptiveType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ThinkingConfigAdaptiveType_Valid_constants(), x)

def anthropicapi_ThinkingConfigDisabledType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[100n, 105n, 115n, 97n, 98n, 108n, 101n, 100n]]

def anthropicapi_ThinkingConfigDisabledType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ThinkingConfigDisabledType_Valid_constants(), x)

def anthropicapi_ThinkingConfigEnabledType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 110n, 97n, 98n, 108n, 101n, 100n]]

def anthropicapi_ThinkingConfigEnabledType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ThinkingConfigEnabledType_Valid_constants(), x)

def anthropicapi_ThinkingContentBlockDeltaType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 104n, 105n, 110n, 107n, 105n, 110n, 103n, 95n, 100n, 101n, 108n, 116n, 97n]]

def anthropicapi_ThinkingContentBlockDeltaType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ThinkingContentBlockDeltaType_Valid_constants(), x)

def anthropicapi_ThinkingDisplayMode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 109n, 105n, 116n, 116n, 101n, 100n], [115n, 117n, 109n, 109n, 97n, 114n, 105n, 122n, 101n, 100n]]

def anthropicapi_ThinkingDisplayMode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ThinkingDisplayMode_Valid_constants(), x)

def anthropicapi_ToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 117n, 115n, 116n, 111n, 109n]]

def anthropicapi_ToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolType_Valid_constants(), x)

def anthropicapi_ToolChoiceAnyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 110n, 121n]]

def anthropicapi_ToolChoiceAnyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolChoiceAnyType_Valid_constants(), x)

def anthropicapi_ToolChoiceAutoType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 117n, 116n, 111n]]

def anthropicapi_ToolChoiceAutoType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolChoiceAutoType_Valid_constants(), x)

def anthropicapi_ToolChoiceNoneType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[110n, 111n, 110n, 101n]]

def anthropicapi_ToolChoiceNoneType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolChoiceNoneType_Valid_constants(), x)

def anthropicapi_ToolChoiceToolType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n]]

def anthropicapi_ToolChoiceToolType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolChoiceToolType_Valid_constants(), x)

def anthropicapi_ToolSearchToolBM2520251119Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 98n, 109n, 50n, 53n]]

def anthropicapi_ToolSearchToolBM2520251119Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolSearchToolBM2520251119Name_Valid_constants(), x)

def anthropicapi_ToolSearchToolBM2520251119Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 98n, 109n, 50n, 53n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 98n, 109n, 50n, 53n, 95n, 50n, 48n, 50n, 53n, 49n, 49n, 49n, 57n]]

def anthropicapi_ToolSearchToolBM2520251119Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolSearchToolBM2520251119Type_Valid_constants(), x)

def anthropicapi_ToolSearchToolRegex20251119Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 103n, 101n, 120n]]

def anthropicapi_ToolSearchToolRegex20251119Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolSearchToolRegex20251119Name_Valid_constants(), x)

def anthropicapi_ToolSearchToolRegex20251119Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 103n, 101n, 120n], [116n, 111n, 111n, 108n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 116n, 111n, 111n, 108n, 95n, 114n, 101n, 103n, 101n, 120n, 95n, 50n, 48n, 50n, 53n, 49n, 49n, 49n, 57n]]

def anthropicapi_ToolSearchToolRegex20251119Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolSearchToolRegex20251119Type_Valid_constants(), x)

def anthropicapi_ToolSearchToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 101n, 99n, 117n, 116n, 105n, 111n, 110n, 95n, 116n, 105n, 109n, 101n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n]]

def anthropicapi_ToolSearchToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_ToolSearchToolResultErrorCode_Valid_constants(), x)

def anthropicapi_URLImageSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 114n, 108n]]

def anthropicapi_URLImageSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_URLImageSourceType_Valid_constants(), x)

def anthropicapi_URLPDFSourceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[117n, 114n, 108n]]

def anthropicapi_URLPDFSourceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_URLPDFSourceType_Valid_constants(), x)

def anthropicapi_UsageServiceTier_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[98n, 97n, 116n, 99n, 104n], [112n, 114n, 105n, 111n, 114n, 105n, 116n, 121n], [115n, 116n, 97n, 110n, 100n, 97n, 114n, 100n]]

def anthropicapi_UsageServiceTier_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_UsageServiceTier_Valid_constants(), x)

def anthropicapi_UserLocationType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 112n, 112n, 114n, 111n, 120n, 105n, 109n, 97n, 116n, 101n]]

def anthropicapi_UserLocationType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_UserLocationType_Valid_constants(), x)

def anthropicapi_WebFetchToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 116n, 111n, 111n, 95n, 108n, 97n, 114n, 103n, 101n], [105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [109n, 97n, 120n, 95n, 117n, 115n, 101n, 115n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n], [117n, 110n, 115n, 117n, 112n, 112n, 111n, 114n, 116n, 101n, 100n, 95n, 99n, 111n, 110n, 116n, 101n, 110n, 116n, 95n, 116n, 121n, 112n, 101n], [117n, 114n, 108n, 95n, 110n, 111n, 116n, 95n, 97n, 99n, 99n, 101n, 115n, 115n, 105n, 98n, 108n, 101n], [117n, 114n, 108n, 95n, 110n, 111n, 116n, 95n, 97n, 108n, 108n, 111n, 119n, 101n, 100n], [117n, 114n, 108n, 95n, 110n, 111n, 116n, 95n, 105n, 110n, 95n, 112n, 114n, 105n, 111n, 114n, 95n, 99n, 111n, 110n, 116n, 101n, 120n, 116n], [117n, 114n, 108n, 95n, 116n, 111n, 111n, 95n, 108n, 111n, 110n, 103n]]

def anthropicapi_WebFetchToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchToolResultErrorCode_Valid_constants(), x)

def anthropicapi_WebFetchTool20250910Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n]]

def anthropicapi_WebFetchTool20250910Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20250910Name_Valid_constants(), x)

def anthropicapi_WebFetchTool20250910Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 50n, 48n, 50n, 53n, 48n, 57n, 49n, 48n]]

def anthropicapi_WebFetchTool20250910Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20250910Type_Valid_constants(), x)

def anthropicapi_WebFetchTool20260209Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n]]

def anthropicapi_WebFetchTool20260209Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260209Name_Valid_constants(), x)

def anthropicapi_WebFetchTool20260209Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 50n, 48n, 50n, 54n, 48n, 50n, 48n, 57n]]

def anthropicapi_WebFetchTool20260209Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260209Type_Valid_constants(), x)

def anthropicapi_WebFetchTool20260309Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n]]

def anthropicapi_WebFetchTool20260309Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260309Name_Valid_constants(), x)

def anthropicapi_WebFetchTool20260309Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 50n, 48n, 50n, 54n, 48n, 51n, 48n, 57n]]

def anthropicapi_WebFetchTool20260309Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260309Type_Valid_constants(), x)

def anthropicapi_WebFetchTool20260318Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n]]

def anthropicapi_WebFetchTool20260318Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260318Name_Valid_constants(), x)

def anthropicapi_WebFetchTool20260318ResponseInclusion_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 99n, 108n, 117n, 100n, 101n, 100n], [102n, 117n, 108n, 108n]]

def anthropicapi_WebFetchTool20260318ResponseInclusion_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260318ResponseInclusion_Valid_constants(), x)

def anthropicapi_WebFetchTool20260318Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 102n, 101n, 116n, 99n, 104n, 95n, 50n, 48n, 50n, 54n, 48n, 51n, 49n, 56n]]

def anthropicapi_WebFetchTool20260318Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchTool20260318Type_Valid_constants(), x)

def anthropicapi_WebFetchUrlSourceAllType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[97n, 108n, 108n]]

def anthropicapi_WebFetchUrlSourceAllType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchUrlSourceAllType_Valid_constants(), x)

def anthropicapi_WebFetchUrlSourceExceptType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 99n, 101n, 112n, 116n]]

def anthropicapi_WebFetchUrlSourceExceptType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchUrlSourceExceptType_Valid_constants(), x)

def anthropicapi_WebFetchUrlSourceNoneType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[110n, 111n, 110n, 101n]]

def anthropicapi_WebFetchUrlSourceNoneType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchUrlSourceNoneType_Valid_constants(), x)

def anthropicapi_WebFetchUrlSourceOnlyType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[111n, 110n, 108n, 121n]]

def anthropicapi_WebFetchUrlSourceOnlyType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchUrlSourceOnlyType_Valid_constants(), x)

def anthropicapi_WebFetchUrlSourceToolReferenceType_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[116n, 111n, 111n, 108n, 95n, 114n, 101n, 102n, 101n, 114n, 101n, 110n, 99n, 101n]]

def anthropicapi_WebFetchUrlSourceToolReferenceType_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebFetchUrlSourceToolReferenceType_Valid_constants(), x)

def anthropicapi_WebSearchToolResultErrorCode_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[105n, 110n, 118n, 97n, 108n, 105n, 100n, 95n, 116n, 111n, 111n, 108n, 95n, 105n, 110n, 112n, 117n, 116n], [109n, 97n, 120n, 95n, 117n, 115n, 101n, 115n, 95n, 101n, 120n, 99n, 101n, 101n, 100n, 101n, 100n], [113n, 117n, 101n, 114n, 121n, 95n, 116n, 111n, 111n, 95n, 108n, 111n, 110n, 103n], [114n, 101n, 113n, 117n, 101n, 115n, 116n, 95n, 116n, 111n, 111n, 95n, 108n, 97n, 114n, 103n, 101n], [116n, 111n, 111n, 95n, 109n, 97n, 110n, 121n, 95n, 114n, 101n, 113n, 117n, 101n, 115n, 116n, 115n], [117n, 110n, 97n, 118n, 97n, 105n, 108n, 97n, 98n, 108n, 101n]]

def anthropicapi_WebSearchToolResultErrorCode_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchToolResultErrorCode_Valid_constants(), x)

def anthropicapi_WebSearchTool20250305Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def anthropicapi_WebSearchTool20250305Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20250305Name_Valid_constants(), x)

def anthropicapi_WebSearchTool20250305Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 50n, 48n, 50n, 53n, 48n, 51n, 48n, 53n]]

def anthropicapi_WebSearchTool20250305Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20250305Type_Valid_constants(), x)

def anthropicapi_WebSearchTool20260209Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def anthropicapi_WebSearchTool20260209Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20260209Name_Valid_constants(), x)

def anthropicapi_WebSearchTool20260209Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 50n, 48n, 50n, 54n, 48n, 50n, 48n, 57n]]

def anthropicapi_WebSearchTool20260209Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20260209Type_Valid_constants(), x)

def anthropicapi_WebSearchTool20260318Name_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n]]

def anthropicapi_WebSearchTool20260318Name_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20260318Name_Valid_constants(), x)

def anthropicapi_WebSearchTool20260318ResponseInclusion_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[101n, 120n, 99n, 108n, 117n, 100n, 101n, 100n], [102n, 117n, 108n, 108n]]

def anthropicapi_WebSearchTool20260318ResponseInclusion_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20260318ResponseInclusion_Valid_constants(), x)

def anthropicapi_WebSearchTool20260318Type_Valid_constants() -> List<&2, List<&2, Nat>>:
  [[119n, 101n, 98n, 95n, 115n, 101n, 97n, 114n, 99n, 104n, 95n, 50n, 48n, 50n, 54n, 48n, 51n, 49n, 56n]]

def anthropicapi_WebSearchTool20260318Type_Valid(+x: List<&2, Nat>) -> Bool:
  H.members(anthropicapi_WebSearchTool20260318Type_Valid_constants(), x)


import Base
import ../enums/semantics.bend as M
import ./helpers.bend as H
import ./SOURCE.bend as S
import ./POLICY.bend as P

law branch_or:
  for +b: Bool
  for +r: Bool
  {M.choose(b, True{}, r) == (b || r) : Bool}

law chain_membership:
  for +values: List<&2, List<&2, Nat>>
  for +x: List<&2, Nat>
  {H.chain(values, x) == H.members(values, x) : Bool}

law openaiapi_AdditionalToolsType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_AdditionalToolsType_Valid(x) == P.openaiapi_AdditionalToolsType_Valid(x) : Bool}

law openaiapi_AdditionalToolsItemParamRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_AdditionalToolsItemParamRole_Valid(x) == P.openaiapi_AdditionalToolsItemParamRole_Valid(x) : Bool}

law openaiapi_AdditionalToolsItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_AdditionalToolsItemParamType_Valid(x) == P.openaiapi_AdditionalToolsItemParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchCallOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCallOutputStatus_Valid(x) == P.openaiapi_ApplyPatchCallOutputStatus_Valid(x) : Bool}

law openaiapi_ApplyPatchCallOutputStatusParam_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCallOutputStatusParam_Valid(x) == P.openaiapi_ApplyPatchCallOutputStatusParam_Valid(x) : Bool}

law openaiapi_ApplyPatchCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCallStatus_Valid(x) == P.openaiapi_ApplyPatchCallStatus_Valid(x) : Bool}

law openaiapi_ApplyPatchCallStatusParam_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCallStatusParam_Valid(x) == P.openaiapi_ApplyPatchCallStatusParam_Valid(x) : Bool}

law openaiapi_ApplyPatchCreateFileOperationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCreateFileOperationType_Valid(x) == P.openaiapi_ApplyPatchCreateFileOperationType_Valid(x) : Bool}

law openaiapi_ApplyPatchCreateFileOperationParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchCreateFileOperationParamType_Valid(x) == P.openaiapi_ApplyPatchCreateFileOperationParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchDeleteFileOperationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchDeleteFileOperationType_Valid(x) == P.openaiapi_ApplyPatchDeleteFileOperationType_Valid(x) : Bool}

law openaiapi_ApplyPatchDeleteFileOperationParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchDeleteFileOperationParamType_Valid(x) == P.openaiapi_ApplyPatchDeleteFileOperationParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchToolCallType_Valid(x) == P.openaiapi_ApplyPatchToolCallType_Valid(x) : Bool}

law openaiapi_ApplyPatchToolCallItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchToolCallItemParamType_Valid(x) == P.openaiapi_ApplyPatchToolCallItemParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchToolCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchToolCallOutputType_Valid(x) == P.openaiapi_ApplyPatchToolCallOutputType_Valid(x) : Bool}

law openaiapi_ApplyPatchToolCallOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchToolCallOutputItemParamType_Valid(x) == P.openaiapi_ApplyPatchToolCallOutputItemParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchToolParamType_Valid(x) == P.openaiapi_ApplyPatchToolParamType_Valid(x) : Bool}

law openaiapi_ApplyPatchUpdateFileOperationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchUpdateFileOperationType_Valid(x) == P.openaiapi_ApplyPatchUpdateFileOperationType_Valid(x) : Bool}

law openaiapi_ApplyPatchUpdateFileOperationParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApplyPatchUpdateFileOperationParamType_Valid(x) == P.openaiapi_ApplyPatchUpdateFileOperationParamType_Valid(x) : Bool}

law openaiapi_ApproximateLocationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ApproximateLocationType_Valid(x) == P.openaiapi_ApproximateLocationType_Valid(x) : Bool}

law openaiapi_AutoCodeInterpreterToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_AutoCodeInterpreterToolParamType_Valid(x) == P.openaiapi_AutoCodeInterpreterToolParamType_Valid(x) : Bool}

law openaiapi_CallableToolAllowedCaller_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CallableToolAllowedCaller_Valid(x) == P.openaiapi_CallableToolAllowedCaller_Valid(x) : Bool}

law openaiapi_ClickButtonType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ClickButtonType_Valid(x) == P.openaiapi_ClickButtonType_Valid(x) : Bool}

law openaiapi_ClickParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ClickParamType_Valid(x) == P.openaiapi_ClickParamType_Valid(x) : Bool}

law openaiapi_CodeInterpreterOutputImageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CodeInterpreterOutputImageType_Valid(x) == P.openaiapi_CodeInterpreterOutputImageType_Valid(x) : Bool}

law openaiapi_CodeInterpreterOutputLogsType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CodeInterpreterOutputLogsType_Valid(x) == P.openaiapi_CodeInterpreterOutputLogsType_Valid(x) : Bool}

law openaiapi_CodeInterpreterToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CodeInterpreterToolType_Valid(x) == P.openaiapi_CodeInterpreterToolType_Valid(x) : Bool}

law openaiapi_CodeInterpreterToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CodeInterpreterToolCallStatus_Valid(x) == P.openaiapi_CodeInterpreterToolCallStatus_Valid(x) : Bool}

law openaiapi_CodeInterpreterToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CodeInterpreterToolCallType_Valid(x) == P.openaiapi_CodeInterpreterToolCallType_Valid(x) : Bool}

law openaiapi_CompactionBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CompactionBodyType_Valid(x) == P.openaiapi_CompactionBodyType_Valid(x) : Bool}

law openaiapi_CompactionSummaryItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CompactionSummaryItemParamType_Valid(x) == P.openaiapi_CompactionSummaryItemParamType_Valid(x) : Bool}

law openaiapi_CompactionTriggerItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CompactionTriggerItemParamType_Valid(x) == P.openaiapi_CompactionTriggerItemParamType_Valid(x) : Bool}

law openaiapi_ComparisonFilterType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComparisonFilterType_Valid(x) == P.openaiapi_ComparisonFilterType_Valid(x) : Bool}

law openaiapi_CompoundFilterType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CompoundFilterType_Valid(x) == P.openaiapi_CompoundFilterType_Valid(x) : Bool}

law openaiapi_ComputerCallOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerCallOutputItemParamType_Valid(x) == P.openaiapi_ComputerCallOutputItemParamType_Valid(x) : Bool}

law openaiapi_ComputerCallOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerCallOutputStatus_Valid(x) == P.openaiapi_ComputerCallOutputStatus_Valid(x) : Bool}

law openaiapi_ComputerEnvironment_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerEnvironment_Valid(x) == P.openaiapi_ComputerEnvironment_Valid(x) : Bool}

law openaiapi_ComputerScreenshotImageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerScreenshotImageType_Valid(x) == P.openaiapi_ComputerScreenshotImageType_Valid(x) : Bool}

law openaiapi_ComputerToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolType_Valid(x) == P.openaiapi_ComputerToolType_Valid(x) : Bool}

law openaiapi_ComputerToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolCallStatus_Valid(x) == P.openaiapi_ComputerToolCallStatus_Valid(x) : Bool}

law openaiapi_ComputerToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolCallType_Valid(x) == P.openaiapi_ComputerToolCallType_Valid(x) : Bool}

law openaiapi_ComputerToolCallOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolCallOutputStatus_Valid(x) == P.openaiapi_ComputerToolCallOutputStatus_Valid(x) : Bool}

law openaiapi_ComputerToolCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolCallOutputType_Valid(x) == P.openaiapi_ComputerToolCallOutputType_Valid(x) : Bool}

law openaiapi_ComputerToolCallOutputResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerToolCallOutputResourceType_Valid(x) == P.openaiapi_ComputerToolCallOutputResourceType_Valid(x) : Bool}

law openaiapi_ComputerUsePreviewToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ComputerUsePreviewToolType_Valid(x) == P.openaiapi_ComputerUsePreviewToolType_Valid(x) : Bool}

law openaiapi_ContainerAutoParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerAutoParamType_Valid(x) == P.openaiapi_ContainerAutoParamType_Valid(x) : Bool}

law openaiapi_ContainerFileCitationBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerFileCitationBodyType_Valid(x) == P.openaiapi_ContainerFileCitationBodyType_Valid(x) : Bool}

law openaiapi_ContainerMemoryLimit_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerMemoryLimit_Valid(x) == P.openaiapi_ContainerMemoryLimit_Valid(x) : Bool}

law openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid(x) == P.openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid(x) : Bool}

law openaiapi_ContainerNetworkPolicyDisabledParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerNetworkPolicyDisabledParamType_Valid(x) == P.openaiapi_ContainerNetworkPolicyDisabledParamType_Valid(x) : Bool}

law openaiapi_ContainerReferenceParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerReferenceParamType_Valid(x) == P.openaiapi_ContainerReferenceParamType_Valid(x) : Bool}

law openaiapi_ContainerReferenceResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ContainerReferenceResourceType_Valid(x) == P.openaiapi_ContainerReferenceResourceType_Valid(x) : Bool}

law openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid(x) == P.openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid(x) : Bool}

law openaiapi_CreateResponsePromptCacheRetention_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CreateResponsePromptCacheRetention_Valid(x) == P.openaiapi_CreateResponsePromptCacheRetention_Valid(x) : Bool}

law openaiapi_CreateResponseTruncation_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CreateResponseTruncation_Valid(x) == P.openaiapi_CreateResponseTruncation_Valid(x) : Bool}

law openaiapi_CustomGrammarFormatParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomGrammarFormatParamType_Valid(x) == P.openaiapi_CustomGrammarFormatParamType_Valid(x) : Bool}

law openaiapi_CustomTextFormatParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomTextFormatParamType_Valid(x) == P.openaiapi_CustomTextFormatParamType_Valid(x) : Bool}

law openaiapi_CustomToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomToolCallType_Valid(x) == P.openaiapi_CustomToolCallType_Valid(x) : Bool}

law openaiapi_CustomToolCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomToolCallOutputType_Valid(x) == P.openaiapi_CustomToolCallOutputType_Valid(x) : Bool}

law openaiapi_CustomToolCallOutputResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomToolCallOutputResourceType_Valid(x) == P.openaiapi_CustomToolCallOutputResourceType_Valid(x) : Bool}

law openaiapi_CustomToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_CustomToolParamType_Valid(x) == P.openaiapi_CustomToolParamType_Valid(x) : Bool}

law openaiapi_DetailEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_DetailEnum_Valid(x) == P.openaiapi_DetailEnum_Valid(x) : Bool}

law openaiapi_DirectToolCallCallerType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_DirectToolCallCallerType_Valid(x) == P.openaiapi_DirectToolCallCallerType_Valid(x) : Bool}

law openaiapi_DirectToolCallCallerParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_DirectToolCallCallerParamType_Valid(x) == P.openaiapi_DirectToolCallCallerParamType_Valid(x) : Bool}

law openaiapi_DoubleClickActionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_DoubleClickActionType_Valid(x) == P.openaiapi_DoubleClickActionType_Valid(x) : Bool}

law openaiapi_DragParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_DragParamType_Valid(x) == P.openaiapi_DragParamType_Valid(x) : Bool}

law openaiapi_EasyInputMessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_EasyInputMessageRole_Valid(x) == P.openaiapi_EasyInputMessageRole_Valid(x) : Bool}

law openaiapi_EasyInputMessageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_EasyInputMessageType_Valid(x) == P.openaiapi_EasyInputMessageType_Valid(x) : Bool}

law openaiapi_FileCitationBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileCitationBodyType_Valid(x) == P.openaiapi_FileCitationBodyType_Valid(x) : Bool}

law openaiapi_FileDetailEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileDetailEnum_Valid(x) == P.openaiapi_FileDetailEnum_Valid(x) : Bool}

law openaiapi_FileInputDetail_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileInputDetail_Valid(x) == P.openaiapi_FileInputDetail_Valid(x) : Bool}

law openaiapi_FilePathType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FilePathType_Valid(x) == P.openaiapi_FilePathType_Valid(x) : Bool}

law openaiapi_FileSearchToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileSearchToolType_Valid(x) == P.openaiapi_FileSearchToolType_Valid(x) : Bool}

law openaiapi_FileSearchToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileSearchToolCallStatus_Valid(x) == P.openaiapi_FileSearchToolCallStatus_Valid(x) : Bool}

law openaiapi_FileSearchToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FileSearchToolCallType_Valid(x) == P.openaiapi_FileSearchToolCallType_Valid(x) : Bool}

law openaiapi_FunctionCallItemStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionCallItemStatus_Valid(x) == P.openaiapi_FunctionCallItemStatus_Valid(x) : Bool}

law openaiapi_FunctionCallOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionCallOutputItemParamType_Valid(x) == P.openaiapi_FunctionCallOutputItemParamType_Valid(x) : Bool}

law openaiapi_FunctionCallOutputStatusEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionCallOutputStatusEnum_Valid(x) == P.openaiapi_FunctionCallOutputStatusEnum_Valid(x) : Bool}

law openaiapi_FunctionCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionCallStatus_Valid(x) == P.openaiapi_FunctionCallStatus_Valid(x) : Bool}

law openaiapi_FunctionShellCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallType_Valid(x) == P.openaiapi_FunctionShellCallType_Valid(x) : Bool}

law openaiapi_FunctionShellCallItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallItemParamType_Valid(x) == P.openaiapi_FunctionShellCallItemParamType_Valid(x) : Bool}

law openaiapi_FunctionShellCallItemStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallItemStatus_Valid(x) == P.openaiapi_FunctionShellCallItemStatus_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputType_Valid(x) == P.openaiapi_FunctionShellCallOutputType_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputExitOutcomeType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputExitOutcomeType_Valid(x) == P.openaiapi_FunctionShellCallOutputExitOutcomeType_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid(x) == P.openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputItemParamType_Valid(x) == P.openaiapi_FunctionShellCallOutputItemParamType_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputStatusEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputStatusEnum_Valid(x) == P.openaiapi_FunctionShellCallOutputStatusEnum_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid(x) == P.openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid(x) : Bool}

law openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid(x) == P.openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid(x) : Bool}

law openaiapi_FunctionShellCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellCallStatus_Valid(x) == P.openaiapi_FunctionShellCallStatus_Valid(x) : Bool}

law openaiapi_FunctionShellToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionShellToolParamType_Valid(x) == P.openaiapi_FunctionShellToolParamType_Valid(x) : Bool}

law openaiapi_FunctionToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolType_Valid(x) == P.openaiapi_FunctionToolType_Valid(x) : Bool}

law openaiapi_FunctionToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolCallStatus_Valid(x) == P.openaiapi_FunctionToolCallStatus_Valid(x) : Bool}

law openaiapi_FunctionToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolCallType_Valid(x) == P.openaiapi_FunctionToolCallType_Valid(x) : Bool}

law openaiapi_FunctionToolCallOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolCallOutputStatus_Valid(x) == P.openaiapi_FunctionToolCallOutputStatus_Valid(x) : Bool}

law openaiapi_FunctionToolCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolCallOutputType_Valid(x) == P.openaiapi_FunctionToolCallOutputType_Valid(x) : Bool}

law openaiapi_FunctionToolCallOutputResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolCallOutputResourceType_Valid(x) == P.openaiapi_FunctionToolCallOutputResourceType_Valid(x) : Bool}

law openaiapi_FunctionToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_FunctionToolParamType_Valid(x) == P.openaiapi_FunctionToolParamType_Valid(x) : Bool}

law openaiapi_GrammarSyntax1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_GrammarSyntax1_Valid(x) == P.openaiapi_GrammarSyntax1_Valid(x) : Bool}

law openaiapi_ImageDetail_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageDetail_Valid(x) == P.openaiapi_ImageDetail_Valid(x) : Bool}

law openaiapi_ImageGenActionEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenActionEnum_Valid(x) == P.openaiapi_ImageGenActionEnum_Valid(x) : Bool}

law openaiapi_ImageGenToolBackground_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolBackground_Valid(x) == P.openaiapi_ImageGenToolBackground_Valid(x) : Bool}

law openaiapi_ImageGenToolModel1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolModel1_Valid(x) == P.openaiapi_ImageGenToolModel1_Valid(x) : Bool}

law openaiapi_ImageGenToolModeration_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolModeration_Valid(x) == P.openaiapi_ImageGenToolModeration_Valid(x) : Bool}

law openaiapi_ImageGenToolOutputFormat_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolOutputFormat_Valid(x) == P.openaiapi_ImageGenToolOutputFormat_Valid(x) : Bool}

law openaiapi_ImageGenToolQuality_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolQuality_Valid(x) == P.openaiapi_ImageGenToolQuality_Valid(x) : Bool}

law openaiapi_ImageGenToolSize1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolSize1_Valid(x) == P.openaiapi_ImageGenToolSize1_Valid(x) : Bool}

law openaiapi_ImageGenToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolType_Valid(x) == P.openaiapi_ImageGenToolType_Valid(x) : Bool}

law openaiapi_ImageGenToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolCallStatus_Valid(x) == P.openaiapi_ImageGenToolCallStatus_Valid(x) : Bool}

law openaiapi_ImageGenToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ImageGenToolCallType_Valid(x) == P.openaiapi_ImageGenToolCallType_Valid(x) : Bool}

law openaiapi_IncludeEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_IncludeEnum_Valid(x) == P.openaiapi_IncludeEnum_Valid(x) : Bool}

law openaiapi_InlineSkillParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InlineSkillParamType_Valid(x) == P.openaiapi_InlineSkillParamType_Valid(x) : Bool}

law openaiapi_InlineSkillSourceParamMediaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InlineSkillSourceParamMediaType_Valid(x) == P.openaiapi_InlineSkillSourceParamMediaType_Valid(x) : Bool}

law openaiapi_InlineSkillSourceParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InlineSkillSourceParamType_Valid(x) == P.openaiapi_InlineSkillSourceParamType_Valid(x) : Bool}

law openaiapi_InputFidelity_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputFidelity_Valid(x) == P.openaiapi_InputFidelity_Valid(x) : Bool}

law openaiapi_InputFileContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputFileContentType_Valid(x) == P.openaiapi_InputFileContentType_Valid(x) : Bool}

law openaiapi_InputFileContentParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputFileContentParamType_Valid(x) == P.openaiapi_InputFileContentParamType_Valid(x) : Bool}

law openaiapi_InputImageContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputImageContentType_Valid(x) == P.openaiapi_InputImageContentType_Valid(x) : Bool}

law openaiapi_InputImageContentParamAutoParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputImageContentParamAutoParamType_Valid(x) == P.openaiapi_InputImageContentParamAutoParamType_Valid(x) : Bool}

law openaiapi_InputMessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputMessageRole_Valid(x) == P.openaiapi_InputMessageRole_Valid(x) : Bool}

law openaiapi_InputMessageStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputMessageStatus_Valid(x) == P.openaiapi_InputMessageStatus_Valid(x) : Bool}

law openaiapi_InputMessageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputMessageType_Valid(x) == P.openaiapi_InputMessageType_Valid(x) : Bool}

law openaiapi_InputTextContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputTextContentType_Valid(x) == P.openaiapi_InputTextContentType_Valid(x) : Bool}

law openaiapi_InputTextContentParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_InputTextContentParamType_Valid(x) == P.openaiapi_InputTextContentParamType_Valid(x) : Bool}

law openaiapi_ItemReferenceParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ItemReferenceParamType_Valid(x) == P.openaiapi_ItemReferenceParamType_Valid(x) : Bool}

law openaiapi_KeyPressActionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_KeyPressActionType_Valid(x) == P.openaiapi_KeyPressActionType_Valid(x) : Bool}

law openaiapi_LocalEnvironmentParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalEnvironmentParamType_Valid(x) == P.openaiapi_LocalEnvironmentParamType_Valid(x) : Bool}

law openaiapi_LocalEnvironmentResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalEnvironmentResourceType_Valid(x) == P.openaiapi_LocalEnvironmentResourceType_Valid(x) : Bool}

law openaiapi_LocalShellExecActionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellExecActionType_Valid(x) == P.openaiapi_LocalShellExecActionType_Valid(x) : Bool}

law openaiapi_LocalShellToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellToolCallStatus_Valid(x) == P.openaiapi_LocalShellToolCallStatus_Valid(x) : Bool}

law openaiapi_LocalShellToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellToolCallType_Valid(x) == P.openaiapi_LocalShellToolCallType_Valid(x) : Bool}

law openaiapi_LocalShellToolCallOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellToolCallOutputStatus_Valid(x) == P.openaiapi_LocalShellToolCallOutputStatus_Valid(x) : Bool}

law openaiapi_LocalShellToolCallOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellToolCallOutputType_Valid(x) == P.openaiapi_LocalShellToolCallOutputType_Valid(x) : Bool}

law openaiapi_LocalShellToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_LocalShellToolParamType_Valid(x) == P.openaiapi_LocalShellToolParamType_Valid(x) : Bool}

law openaiapi_MCPApprovalRequestType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPApprovalRequestType_Valid(x) == P.openaiapi_MCPApprovalRequestType_Valid(x) : Bool}

law openaiapi_MCPApprovalResponseType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPApprovalResponseType_Valid(x) == P.openaiapi_MCPApprovalResponseType_Valid(x) : Bool}

law openaiapi_MCPApprovalResponseResourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPApprovalResponseResourceType_Valid(x) == P.openaiapi_MCPApprovalResponseResourceType_Valid(x) : Bool}

law openaiapi_MCPListToolsType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPListToolsType_Valid(x) == P.openaiapi_MCPListToolsType_Valid(x) : Bool}

law openaiapi_MCPToolConnectorId_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPToolConnectorId_Valid(x) == P.openaiapi_MCPToolConnectorId_Valid(x) : Bool}

law openaiapi_MCPToolRequireApproval1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPToolRequireApproval1_Valid(x) == P.openaiapi_MCPToolRequireApproval1_Valid(x) : Bool}

law openaiapi_MCPToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPToolType_Valid(x) == P.openaiapi_MCPToolType_Valid(x) : Bool}

law openaiapi_MCPToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPToolCallType_Valid(x) == P.openaiapi_MCPToolCallType_Valid(x) : Bool}

law openaiapi_MCPToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MCPToolCallStatus_Valid(x) == P.openaiapi_MCPToolCallStatus_Valid(x) : Bool}

law openaiapi_MessagePhase_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MessagePhase_Valid(x) == P.openaiapi_MessagePhase_Valid(x) : Bool}

law openaiapi_MessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MessageRole_Valid(x) == P.openaiapi_MessageRole_Valid(x) : Bool}

law openaiapi_ModelIdsResponses1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModelIdsResponses1_Valid(x) == P.openaiapi_ModelIdsResponses1_Valid(x) : Bool}

law openaiapi_ModelIdsShared1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModelIdsShared1_Valid(x) == P.openaiapi_ModelIdsShared1_Valid(x) : Bool}

law openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid(x) == P.openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid(x) : Bool}

law openaiapi_ModerationErrorBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModerationErrorBodyType_Valid(x) == P.openaiapi_ModerationErrorBodyType_Valid(x) : Bool}

law openaiapi_ModerationInputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModerationInputType_Valid(x) == P.openaiapi_ModerationInputType_Valid(x) : Bool}

law openaiapi_ModerationMode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModerationMode_Valid(x) == P.openaiapi_ModerationMode_Valid(x) : Bool}

law openaiapi_ModerationResultBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ModerationResultBodyType_Valid(x) == P.openaiapi_ModerationResultBodyType_Valid(x) : Bool}

law openaiapi_MoveParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_MoveParamType_Valid(x) == P.openaiapi_MoveParamType_Valid(x) : Bool}

law openaiapi_NamespaceToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_NamespaceToolParamType_Valid(x) == P.openaiapi_NamespaceToolParamType_Valid(x) : Bool}

law openaiapi_OutputMessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_OutputMessageRole_Valid(x) == P.openaiapi_OutputMessageRole_Valid(x) : Bool}

law openaiapi_OutputMessageStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_OutputMessageStatus_Valid(x) == P.openaiapi_OutputMessageStatus_Valid(x) : Bool}

law openaiapi_OutputMessageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_OutputMessageType_Valid(x) == P.openaiapi_OutputMessageType_Valid(x) : Bool}

law openaiapi_OutputTextContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_OutputTextContentType_Valid(x) == P.openaiapi_OutputTextContentType_Valid(x) : Bool}

law openaiapi_ProgramType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramType_Valid(x) == P.openaiapi_ProgramType_Valid(x) : Bool}

law openaiapi_ProgramItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramItemParamType_Valid(x) == P.openaiapi_ProgramItemParamType_Valid(x) : Bool}

law openaiapi_ProgramOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramOutputType_Valid(x) == P.openaiapi_ProgramOutputType_Valid(x) : Bool}

law openaiapi_ProgramOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramOutputItemParamType_Valid(x) == P.openaiapi_ProgramOutputItemParamType_Valid(x) : Bool}

law openaiapi_ProgramOutputItemStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramOutputItemStatus_Valid(x) == P.openaiapi_ProgramOutputItemStatus_Valid(x) : Bool}

law openaiapi_ProgramOutputStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramOutputStatus_Valid(x) == P.openaiapi_ProgramOutputStatus_Valid(x) : Bool}

law openaiapi_ProgramToolCallCallerType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramToolCallCallerType_Valid(x) == P.openaiapi_ProgramToolCallCallerType_Valid(x) : Bool}

law openaiapi_ProgramToolCallCallerParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgramToolCallCallerParamType_Valid(x) == P.openaiapi_ProgramToolCallCallerParamType_Valid(x) : Bool}

law openaiapi_ProgrammaticToolCallingParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ProgrammaticToolCallingParamType_Valid(x) == P.openaiapi_ProgrammaticToolCallingParamType_Valid(x) : Bool}

law openaiapi_PromptCacheBreakpointConfigMode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_PromptCacheBreakpointConfigMode_Valid(x) == P.openaiapi_PromptCacheBreakpointConfigMode_Valid(x) : Bool}

law openaiapi_PromptCacheBreakpointParamMode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_PromptCacheBreakpointParamMode_Valid(x) == P.openaiapi_PromptCacheBreakpointParamMode_Valid(x) : Bool}

law openaiapi_PromptCacheModeEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_PromptCacheModeEnum_Valid(x) == P.openaiapi_PromptCacheModeEnum_Valid(x) : Bool}

law openaiapi_PromptCacheTTLEnum_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_PromptCacheTTLEnum_Valid(x) == P.openaiapi_PromptCacheTTLEnum_Valid(x) : Bool}

law openaiapi_RankerVersionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_RankerVersionType_Valid(x) == P.openaiapi_RankerVersionType_Valid(x) : Bool}

law openaiapi_ReasoningContext_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningContext_Valid(x) == P.openaiapi_ReasoningContext_Valid(x) : Bool}

law openaiapi_ReasoningGenerateSummary_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningGenerateSummary_Valid(x) == P.openaiapi_ReasoningGenerateSummary_Valid(x) : Bool}

law openaiapi_ReasoningSummary_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningSummary_Valid(x) == P.openaiapi_ReasoningSummary_Valid(x) : Bool}

law openaiapi_ReasoningItemStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningItemStatus_Valid(x) == P.openaiapi_ReasoningItemStatus_Valid(x) : Bool}

law openaiapi_ReasoningItemType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningItemType_Valid(x) == P.openaiapi_ReasoningItemType_Valid(x) : Bool}

law openaiapi_ReasoningModeEnum1_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningModeEnum1_Valid(x) == P.openaiapi_ReasoningModeEnum1_Valid(x) : Bool}

law openaiapi_ReasoningTextContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ReasoningTextContentType_Valid(x) == P.openaiapi_ReasoningTextContentType_Valid(x) : Bool}

law openaiapi_RefusalContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_RefusalContentType_Valid(x) == P.openaiapi_RefusalContentType_Valid(x) : Bool}

law openaiapi_ResponseIncompleteDetailsReason_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseIncompleteDetailsReason_Valid(x) == P.openaiapi_ResponseIncompleteDetailsReason_Valid(x) : Bool}

law openaiapi_ResponseObject_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseObject_Valid(x) == P.openaiapi_ResponseObject_Valid(x) : Bool}

law openaiapi_ResponsePromptCacheRetention_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponsePromptCacheRetention_Valid(x) == P.openaiapi_ResponsePromptCacheRetention_Valid(x) : Bool}

law openaiapi_ResponseStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseStatus_Valid(x) == P.openaiapi_ResponseStatus_Valid(x) : Bool}

law openaiapi_ResponseTruncation_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseTruncation_Valid(x) == P.openaiapi_ResponseTruncation_Valid(x) : Bool}

law openaiapi_ResponseAudioDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseAudioDeltaEventType_Valid(x) == P.openaiapi_ResponseAudioDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseAudioDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseAudioDoneEventType_Valid(x) == P.openaiapi_ResponseAudioDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseAudioTranscriptDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseAudioTranscriptDeltaEventType_Valid(x) == P.openaiapi_ResponseAudioTranscriptDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseAudioTranscriptDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseAudioTranscriptDoneEventType_Valid(x) == P.openaiapi_ResponseAudioTranscriptDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid(x) == P.openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid(x) == P.openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid(x) == P.openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid(x) == P.openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid(x) == P.openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid(x) : Bool}

law openaiapi_ResponseCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCompletedEventType_Valid(x) == P.openaiapi_ResponseCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseContentPartAddedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseContentPartAddedEventType_Valid(x) == P.openaiapi_ResponseContentPartAddedEventType_Valid(x) : Bool}

law openaiapi_ResponseContentPartDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseContentPartDoneEventType_Valid(x) == P.openaiapi_ResponseContentPartDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseCreatedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCreatedEventType_Valid(x) == P.openaiapi_ResponseCreatedEventType_Valid(x) : Bool}

law openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid(x) == P.openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseCustomToolCallInputDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseCustomToolCallInputDoneEventType_Valid(x) == P.openaiapi_ResponseCustomToolCallInputDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseErrorCode_Valid(x) == P.openaiapi_ResponseErrorCode_Valid(x) : Bool}

law openaiapi_ResponseErrorEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseErrorEventType_Valid(x) == P.openaiapi_ResponseErrorEventType_Valid(x) : Bool}

law openaiapi_ResponseFailedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFailedEventType_Valid(x) == P.openaiapi_ResponseFailedEventType_Valid(x) : Bool}

law openaiapi_ResponseFileSearchCallCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFileSearchCallCompletedEventType_Valid(x) == P.openaiapi_ResponseFileSearchCallCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseFileSearchCallInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFileSearchCallInProgressEventType_Valid(x) == P.openaiapi_ResponseFileSearchCallInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseFileSearchCallSearchingEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFileSearchCallSearchingEventType_Valid(x) == P.openaiapi_ResponseFileSearchCallSearchingEventType_Valid(x) : Bool}

law openaiapi_ResponseFormatJsonObjectType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFormatJsonObjectType_Valid(x) == P.openaiapi_ResponseFormatJsonObjectType_Valid(x) : Bool}

law openaiapi_ResponseFormatTextType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFormatTextType_Valid(x) == P.openaiapi_ResponseFormatTextType_Valid(x) : Bool}

law openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid(x) == P.openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid(x) == P.openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseImageGenCallCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseImageGenCallCompletedEventType_Valid(x) == P.openaiapi_ResponseImageGenCallCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseImageGenCallGeneratingEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseImageGenCallGeneratingEventType_Valid(x) == P.openaiapi_ResponseImageGenCallGeneratingEventType_Valid(x) : Bool}

law openaiapi_ResponseImageGenCallInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseImageGenCallInProgressEventType_Valid(x) == P.openaiapi_ResponseImageGenCallInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseImageGenCallPartialImageEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseImageGenCallPartialImageEventType_Valid(x) == P.openaiapi_ResponseImageGenCallPartialImageEventType_Valid(x) : Bool}

law openaiapi_ResponseInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseInProgressEventType_Valid(x) == P.openaiapi_ResponseInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseIncompleteEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseIncompleteEventType_Valid(x) == P.openaiapi_ResponseIncompleteEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid(x) == P.openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid(x) == P.openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPCallCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPCallCompletedEventType_Valid(x) == P.openaiapi_ResponseMCPCallCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPCallFailedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPCallFailedEventType_Valid(x) == P.openaiapi_ResponseMCPCallFailedEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPCallInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPCallInProgressEventType_Valid(x) == P.openaiapi_ResponseMCPCallInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPListToolsCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPListToolsCompletedEventType_Valid(x) == P.openaiapi_ResponseMCPListToolsCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPListToolsFailedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPListToolsFailedEventType_Valid(x) == P.openaiapi_ResponseMCPListToolsFailedEventType_Valid(x) : Bool}

law openaiapi_ResponseMCPListToolsInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseMCPListToolsInProgressEventType_Valid(x) == P.openaiapi_ResponseMCPListToolsInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseOutputItemAddedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseOutputItemAddedEventType_Valid(x) == P.openaiapi_ResponseOutputItemAddedEventType_Valid(x) : Bool}

law openaiapi_ResponseOutputItemDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseOutputItemDoneEventType_Valid(x) == P.openaiapi_ResponseOutputItemDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid(x) == P.openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid(x) : Bool}

law openaiapi_ResponseQueuedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseQueuedEventType_Valid(x) == P.openaiapi_ResponseQueuedEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid(x) == P.openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid(x) == P.openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid(x) == P.openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid(x) == P.openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid(x) == P.openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid(x) == P.openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid(x) == P.openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningTextDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningTextDeltaEventType_Valid(x) == P.openaiapi_ResponseReasoningTextDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseReasoningTextDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseReasoningTextDoneEventType_Valid(x) == P.openaiapi_ResponseReasoningTextDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseRefusalDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseRefusalDeltaEventType_Valid(x) == P.openaiapi_ResponseRefusalDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseRefusalDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseRefusalDoneEventType_Valid(x) == P.openaiapi_ResponseRefusalDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseTextDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseTextDeltaEventType_Valid(x) == P.openaiapi_ResponseTextDeltaEventType_Valid(x) : Bool}

law openaiapi_ResponseTextDoneEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseTextDoneEventType_Valid(x) == P.openaiapi_ResponseTextDoneEventType_Valid(x) : Bool}

law openaiapi_ResponseWebSearchCallCompletedEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseWebSearchCallCompletedEventType_Valid(x) == P.openaiapi_ResponseWebSearchCallCompletedEventType_Valid(x) : Bool}

law openaiapi_ResponseWebSearchCallInProgressEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseWebSearchCallInProgressEventType_Valid(x) == P.openaiapi_ResponseWebSearchCallInProgressEventType_Valid(x) : Bool}

law openaiapi_ResponseWebSearchCallSearchingEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ResponseWebSearchCallSearchingEventType_Valid(x) == P.openaiapi_ResponseWebSearchCallSearchingEventType_Valid(x) : Bool}

law openaiapi_ScreenshotParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ScreenshotParamType_Valid(x) == P.openaiapi_ScreenshotParamType_Valid(x) : Bool}

law openaiapi_ScrollParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ScrollParamType_Valid(x) == P.openaiapi_ScrollParamType_Valid(x) : Bool}

law openaiapi_SearchContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SearchContentType_Valid(x) == P.openaiapi_SearchContentType_Valid(x) : Bool}

law openaiapi_SearchContextSize_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SearchContextSize_Valid(x) == P.openaiapi_SearchContextSize_Valid(x) : Bool}

law openaiapi_SkillReferenceParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SkillReferenceParamType_Valid(x) == P.openaiapi_SkillReferenceParamType_Valid(x) : Bool}

law openaiapi_SpecificApplyPatchParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SpecificApplyPatchParamType_Valid(x) == P.openaiapi_SpecificApplyPatchParamType_Valid(x) : Bool}

law openaiapi_SpecificFunctionShellParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SpecificFunctionShellParamType_Valid(x) == P.openaiapi_SpecificFunctionShellParamType_Valid(x) : Bool}

law openaiapi_SpecificProgrammaticToolCallingParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SpecificProgrammaticToolCallingParamType_Valid(x) == P.openaiapi_SpecificProgrammaticToolCallingParamType_Valid(x) : Bool}

law openaiapi_SummaryTextContentType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_SummaryTextContentType_Valid(x) == P.openaiapi_SummaryTextContentType_Valid(x) : Bool}

law openaiapi_TextResponseFormatJsonSchemaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_TextResponseFormatJsonSchemaType_Valid(x) == P.openaiapi_TextResponseFormatJsonSchemaType_Valid(x) : Bool}

law openaiapi_ToolChoiceAllowedMode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceAllowedMode_Valid(x) == P.openaiapi_ToolChoiceAllowedMode_Valid(x) : Bool}

law openaiapi_ToolChoiceAllowedType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceAllowedType_Valid(x) == P.openaiapi_ToolChoiceAllowedType_Valid(x) : Bool}

law openaiapi_ToolChoiceCustomType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceCustomType_Valid(x) == P.openaiapi_ToolChoiceCustomType_Valid(x) : Bool}

law openaiapi_ToolChoiceFunctionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceFunctionType_Valid(x) == P.openaiapi_ToolChoiceFunctionType_Valid(x) : Bool}

law openaiapi_ToolChoiceMCPType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceMCPType_Valid(x) == P.openaiapi_ToolChoiceMCPType_Valid(x) : Bool}

law openaiapi_ToolChoiceOptions_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceOptions_Valid(x) == P.openaiapi_ToolChoiceOptions_Valid(x) : Bool}

law openaiapi_ToolChoiceTypesType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolChoiceTypesType_Valid(x) == P.openaiapi_ToolChoiceTypesType_Valid(x) : Bool}

law openaiapi_ToolSearchCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchCallType_Valid(x) == P.openaiapi_ToolSearchCallType_Valid(x) : Bool}

law openaiapi_ToolSearchCallItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchCallItemParamType_Valid(x) == P.openaiapi_ToolSearchCallItemParamType_Valid(x) : Bool}

law openaiapi_ToolSearchExecutionType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchExecutionType_Valid(x) == P.openaiapi_ToolSearchExecutionType_Valid(x) : Bool}

law openaiapi_ToolSearchOutputType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchOutputType_Valid(x) == P.openaiapi_ToolSearchOutputType_Valid(x) : Bool}

law openaiapi_ToolSearchOutputItemParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchOutputItemParamType_Valid(x) == P.openaiapi_ToolSearchOutputItemParamType_Valid(x) : Bool}

law openaiapi_ToolSearchToolParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_ToolSearchToolParamType_Valid(x) == P.openaiapi_ToolSearchToolParamType_Valid(x) : Bool}

law openaiapi_TypeParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_TypeParamType_Valid(x) == P.openaiapi_TypeParamType_Valid(x) : Bool}

law openaiapi_UrlCitationBodyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_UrlCitationBodyType_Valid(x) == P.openaiapi_UrlCitationBodyType_Valid(x) : Bool}

law openaiapi_WaitParamType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WaitParamType_Valid(x) == P.openaiapi_WaitParamType_Valid(x) : Bool}

law openaiapi_WebSearchActionFindType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchActionFindType_Valid(x) == P.openaiapi_WebSearchActionFindType_Valid(x) : Bool}

law openaiapi_WebSearchActionOpenPageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchActionOpenPageType_Valid(x) == P.openaiapi_WebSearchActionOpenPageType_Valid(x) : Bool}

law openaiapi_WebSearchActionSearchSourcesType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchActionSearchSourcesType_Valid(x) == P.openaiapi_WebSearchActionSearchSourcesType_Valid(x) : Bool}

law openaiapi_WebSearchActionSearchType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchActionSearchType_Valid(x) == P.openaiapi_WebSearchActionSearchType_Valid(x) : Bool}

law openaiapi_WebSearchApproximateLocationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchApproximateLocationType_Valid(x) == P.openaiapi_WebSearchApproximateLocationType_Valid(x) : Bool}

law openaiapi_WebSearchPreviewToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchPreviewToolType_Valid(x) == P.openaiapi_WebSearchPreviewToolType_Valid(x) : Bool}

law openaiapi_WebSearchToolSearchContextSize_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchToolSearchContextSize_Valid(x) == P.openaiapi_WebSearchToolSearchContextSize_Valid(x) : Bool}

law openaiapi_WebSearchToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchToolType_Valid(x) == P.openaiapi_WebSearchToolType_Valid(x) : Bool}

law openaiapi_WebSearchToolCallStatus_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchToolCallStatus_Valid(x) == P.openaiapi_WebSearchToolCallStatus_Valid(x) : Bool}

law openaiapi_WebSearchToolCallType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.openaiapi_WebSearchToolCallType_Valid(x) == P.openaiapi_WebSearchToolCallType_Valid(x) : Bool}

law anthropicapi_APIErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_APIErrorType_Valid(x) == P.anthropicapi_APIErrorType_Valid(x) : Bool}

law anthropicapi_AllowedCaller_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_AllowedCaller_Valid(x) == P.anthropicapi_AllowedCaller_Valid(x) : Bool}

law anthropicapi_AuthenticationErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_AuthenticationErrorType_Valid(x) == P.anthropicapi_AuthenticationErrorType_Valid(x) : Bool}

law anthropicapi_Base64ImageSourceMediaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_Base64ImageSourceMediaType_Valid(x) == P.anthropicapi_Base64ImageSourceMediaType_Valid(x) : Bool}

law anthropicapi_Base64ImageSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_Base64ImageSourceType_Valid(x) == P.anthropicapi_Base64ImageSourceType_Valid(x) : Bool}

law anthropicapi_Base64PDFSourceMediaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_Base64PDFSourceMediaType_Valid(x) == P.anthropicapi_Base64PDFSourceMediaType_Valid(x) : Bool}

law anthropicapi_Base64PDFSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_Base64PDFSourceType_Valid(x) == P.anthropicapi_Base64PDFSourceType_Valid(x) : Bool}

law anthropicapi_BashCodeExecutionToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BashCodeExecutionToolResultErrorCode_Valid(x) == P.anthropicapi_BashCodeExecutionToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_BashTool20250124Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BashTool20250124Name_Valid(x) == P.anthropicapi_BashTool20250124Name_Valid(x) : Bool}

law anthropicapi_BashTool20250124Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BashTool20250124Type_Valid(x) == P.anthropicapi_BashTool20250124Type_Valid(x) : Bool}

law anthropicapi_BillingErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BillingErrorType_Valid(x) == P.anthropicapi_BillingErrorType_Valid(x) : Bool}

law anthropicapi_BrowserStateChangeDownloadCompletedType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BrowserStateChangeDownloadCompletedType_Valid(x) == P.anthropicapi_BrowserStateChangeDownloadCompletedType_Valid(x) : Bool}

law anthropicapi_BrowserStateChangeDownloadFailedType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BrowserStateChangeDownloadFailedType_Valid(x) == P.anthropicapi_BrowserStateChangeDownloadFailedType_Valid(x) : Bool}

law anthropicapi_BrowserStateChangeDownloadStartedType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BrowserStateChangeDownloadStartedType_Valid(x) == P.anthropicapi_BrowserStateChangeDownloadStartedType_Valid(x) : Bool}

law anthropicapi_BrowserStateChangeTabOpenedType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BrowserStateChangeTabOpenedType_Valid(x) == P.anthropicapi_BrowserStateChangeTabOpenedType_Valid(x) : Bool}

law anthropicapi_BrowserToolset20260801Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_BrowserToolset20260801Type_Valid(x) == P.anthropicapi_BrowserToolset20260801Type_Valid(x) : Bool}

law anthropicapi_CacheControlEphemeralTtl_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CacheControlEphemeralTtl_Valid(x) == P.anthropicapi_CacheControlEphemeralTtl_Valid(x) : Bool}

law anthropicapi_CacheControlEphemeralType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CacheControlEphemeralType_Valid(x) == P.anthropicapi_CacheControlEphemeralType_Valid(x) : Bool}

law anthropicapi_CitationsDeltaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CitationsDeltaType_Valid(x) == P.anthropicapi_CitationsDeltaType_Valid(x) : Bool}

law anthropicapi_CodeExecutionToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionToolResultErrorCode_Valid(x) == P.anthropicapi_CodeExecutionToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20250522Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20250522Name_Valid(x) == P.anthropicapi_CodeExecutionTool20250522Name_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20250522Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20250522Type_Valid(x) == P.anthropicapi_CodeExecutionTool20250522Type_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20250825Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20250825Name_Valid(x) == P.anthropicapi_CodeExecutionTool20250825Name_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20250825Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20250825Type_Valid(x) == P.anthropicapi_CodeExecutionTool20250825Type_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20260120Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20260120Name_Valid(x) == P.anthropicapi_CodeExecutionTool20260120Name_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20260120Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20260120Type_Valid(x) == P.anthropicapi_CodeExecutionTool20260120Type_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20260521Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20260521Name_Valid(x) == P.anthropicapi_CodeExecutionTool20260521Name_Valid(x) : Bool}

law anthropicapi_CodeExecutionTool20260521Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CodeExecutionTool20260521Type_Valid(x) == P.anthropicapi_CodeExecutionTool20260521Type_Valid(x) : Bool}

law anthropicapi_ComputerToolset20260801Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ComputerToolset20260801Type_Valid(x) == P.anthropicapi_ComputerToolset20260801Type_Valid(x) : Bool}

law anthropicapi_ContainerSkillType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ContainerSkillType_Valid(x) == P.anthropicapi_ContainerSkillType_Valid(x) : Bool}

law anthropicapi_ContentBlockDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ContentBlockDeltaEventType_Valid(x) == P.anthropicapi_ContentBlockDeltaEventType_Valid(x) : Bool}

law anthropicapi_ContentBlockSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ContentBlockSourceType_Valid(x) == P.anthropicapi_ContentBlockSourceType_Valid(x) : Bool}

law anthropicapi_ContentBlockStartEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ContentBlockStartEventType_Valid(x) == P.anthropicapi_ContentBlockStartEventType_Valid(x) : Bool}

law anthropicapi_ContentBlockStopEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ContentBlockStopEventType_Valid(x) == P.anthropicapi_ContentBlockStopEventType_Valid(x) : Bool}

law anthropicapi_CreateMessageParamsServiceTier_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_CreateMessageParamsServiceTier_Valid(x) == P.anthropicapi_CreateMessageParamsServiceTier_Valid(x) : Bool}

law anthropicapi_DirectCallerType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_DirectCallerType_Valid(x) == P.anthropicapi_DirectCallerType_Valid(x) : Bool}

law anthropicapi_EffortLevel_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_EffortLevel_Valid(x) == P.anthropicapi_EffortLevel_Valid(x) : Bool}

law anthropicapi_ErrorResponseType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ErrorResponseType_Valid(x) == P.anthropicapi_ErrorResponseType_Valid(x) : Bool}

law anthropicapi_FileDocumentSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_FileDocumentSourceType_Valid(x) == P.anthropicapi_FileDocumentSourceType_Valid(x) : Bool}

law anthropicapi_FileImageSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_FileImageSourceType_Valid(x) == P.anthropicapi_FileImageSourceType_Valid(x) : Bool}

law anthropicapi_GatewayTimeoutErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_GatewayTimeoutErrorType_Valid(x) == P.anthropicapi_GatewayTimeoutErrorType_Valid(x) : Bool}

law anthropicapi_InputJsonContentBlockDeltaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_InputJsonContentBlockDeltaType_Valid(x) == P.anthropicapi_InputJsonContentBlockDeltaType_Valid(x) : Bool}

law anthropicapi_InputMessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_InputMessageRole_Valid(x) == P.anthropicapi_InputMessageRole_Valid(x) : Bool}

law anthropicapi_InputSchemaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_InputSchemaType_Valid(x) == P.anthropicapi_InputSchemaType_Valid(x) : Bool}

law anthropicapi_InvalidRequestErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_InvalidRequestErrorType_Valid(x) == P.anthropicapi_InvalidRequestErrorType_Valid(x) : Bool}

law anthropicapi_JsonOutputFormatType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_JsonOutputFormatType_Valid(x) == P.anthropicapi_JsonOutputFormatType_Valid(x) : Bool}

law anthropicapi_MemoryTool20250818Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MemoryTool20250818Name_Valid(x) == P.anthropicapi_MemoryTool20250818Name_Valid(x) : Bool}

law anthropicapi_MemoryTool20250818Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MemoryTool20250818Type_Valid(x) == P.anthropicapi_MemoryTool20250818Type_Valid(x) : Bool}

law anthropicapi_MessageRole_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MessageRole_Valid(x) == P.anthropicapi_MessageRole_Valid(x) : Bool}

law anthropicapi_MessageType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MessageType_Valid(x) == P.anthropicapi_MessageType_Valid(x) : Bool}

law anthropicapi_MessageDeltaEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MessageDeltaEventType_Valid(x) == P.anthropicapi_MessageDeltaEventType_Valid(x) : Bool}

law anthropicapi_MessageStartEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MessageStartEventType_Valid(x) == P.anthropicapi_MessageStartEventType_Valid(x) : Bool}

law anthropicapi_MessageStopEventType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_MessageStopEventType_Valid(x) == P.anthropicapi_MessageStopEventType_Valid(x) : Bool}

law anthropicapi_NotFoundErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_NotFoundErrorType_Valid(x) == P.anthropicapi_NotFoundErrorType_Valid(x) : Bool}

law anthropicapi_OverloadedErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_OverloadedErrorType_Valid(x) == P.anthropicapi_OverloadedErrorType_Valid(x) : Bool}

law anthropicapi_PermissionErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_PermissionErrorType_Valid(x) == P.anthropicapi_PermissionErrorType_Valid(x) : Bool}

law anthropicapi_PlainTextSourceMediaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_PlainTextSourceMediaType_Valid(x) == P.anthropicapi_PlainTextSourceMediaType_Valid(x) : Bool}

law anthropicapi_PlainTextSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_PlainTextSourceType_Valid(x) == P.anthropicapi_PlainTextSourceType_Valid(x) : Bool}

law anthropicapi_RateLimitErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RateLimitErrorType_Valid(x) == P.anthropicapi_RateLimitErrorType_Valid(x) : Bool}

law anthropicapi_RefusalCategory_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RefusalCategory_Valid(x) == P.anthropicapi_RefusalCategory_Valid(x) : Bool}

law anthropicapi_RefusalStopDetailsType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RefusalStopDetailsType_Valid(x) == P.anthropicapi_RefusalStopDetailsType_Valid(x) : Bool}

law anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid(x) == P.anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid(x) : Bool}

law anthropicapi_RequestBashCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestBashCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_RequestBashCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_RequestBrowserStateBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestBrowserStateBlockType_Valid(x) == P.anthropicapi_RequestBrowserStateBlockType_Valid(x) : Bool}

law anthropicapi_RequestCharLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestCharLocationCitationType_Valid(x) == P.anthropicapi_RequestCharLocationCitationType_Valid(x) : Bool}

law anthropicapi_RequestCodeExecutionOutputBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestCodeExecutionOutputBlockType_Valid(x) == P.anthropicapi_RequestCodeExecutionOutputBlockType_Valid(x) : Bool}

law anthropicapi_RequestCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_RequestCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_RequestCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_RequestCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_RequestContainerUploadBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestContainerUploadBlockType_Valid(x) == P.anthropicapi_RequestContainerUploadBlockType_Valid(x) : Bool}

law anthropicapi_RequestContentBlockLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestContentBlockLocationCitationType_Valid(x) == P.anthropicapi_RequestContentBlockLocationCitationType_Valid(x) : Bool}

law anthropicapi_RequestDocumentBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestDocumentBlockType_Valid(x) == P.anthropicapi_RequestDocumentBlockType_Valid(x) : Bool}

law anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestImageBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestImageBlockType_Valid(x) == P.anthropicapi_RequestImageBlockType_Valid(x) : Bool}

law anthropicapi_RequestImageTransformationsOversizedImage_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestImageTransformationsOversizedImage_Valid(x) == P.anthropicapi_RequestImageTransformationsOversizedImage_Valid(x) : Bool}

law anthropicapi_RequestPageLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestPageLocationCitationType_Valid(x) == P.anthropicapi_RequestPageLocationCitationType_Valid(x) : Bool}

law anthropicapi_RequestRedactedThinkingBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestRedactedThinkingBlockType_Valid(x) == P.anthropicapi_RequestRedactedThinkingBlockType_Valid(x) : Bool}

law anthropicapi_RequestSearchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestSearchResultBlockType_Valid(x) == P.anthropicapi_RequestSearchResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestSearchResultLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestSearchResultLocationCitationType_Valid(x) == P.anthropicapi_RequestSearchResultLocationCitationType_Valid(x) : Bool}

law anthropicapi_RequestServerToolUseBlockName_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestServerToolUseBlockName_Valid(x) == P.anthropicapi_RequestServerToolUseBlockName_Valid(x) : Bool}

law anthropicapi_RequestServerToolUseBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestServerToolUseBlockType_Valid(x) == P.anthropicapi_RequestServerToolUseBlockType_Valid(x) : Bool}

law anthropicapi_RequestTextBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextBlockType_Valid(x) == P.anthropicapi_RequestTextBlockType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid(x) : Bool}

law anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid(x) == P.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestThinkingBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestThinkingBlockType_Valid(x) == P.anthropicapi_RequestThinkingBlockType_Valid(x) : Bool}

law anthropicapi_RequestToolReferenceBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolReferenceBlockType_Valid(x) == P.anthropicapi_RequestToolReferenceBlockType_Valid(x) : Bool}

law anthropicapi_RequestToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolResultBlockType_Valid(x) == P.anthropicapi_RequestToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestToolSearchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolSearchToolResultBlockType_Valid(x) == P.anthropicapi_RequestToolSearchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestToolSearchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolSearchToolResultErrorType_Valid(x) == P.anthropicapi_RequestToolSearchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid(x) == P.anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestToolUseBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestToolUseBlockType_Valid(x) == P.anthropicapi_RequestToolUseBlockType_Valid(x) : Bool}

law anthropicapi_RequestWebFetchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebFetchResultBlockType_Valid(x) == P.anthropicapi_RequestWebFetchResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestWebFetchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebFetchToolResultBlockType_Valid(x) == P.anthropicapi_RequestWebFetchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestWebFetchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebFetchToolResultErrorType_Valid(x) == P.anthropicapi_RequestWebFetchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_RequestWebSearchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebSearchResultBlockType_Valid(x) == P.anthropicapi_RequestWebSearchResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestWebSearchResultLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebSearchResultLocationCitationType_Valid(x) == P.anthropicapi_RequestWebSearchResultLocationCitationType_Valid(x) : Bool}

law anthropicapi_RequestWebSearchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebSearchToolResultBlockType_Valid(x) == P.anthropicapi_RequestWebSearchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_RequestWebSearchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_RequestWebSearchToolResultErrorType_Valid(x) == P.anthropicapi_RequestWebSearchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid(x) == P.anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid(x) : Bool}

law anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseCharLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseCharLocationCitationType_Valid(x) == P.anthropicapi_ResponseCharLocationCitationType_Valid(x) : Bool}

law anthropicapi_ResponseCodeExecutionOutputBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseCodeExecutionOutputBlockType_Valid(x) == P.anthropicapi_ResponseCodeExecutionOutputBlockType_Valid(x) : Bool}

law anthropicapi_ResponseCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_ResponseCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseContainerUploadBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseContainerUploadBlockType_Valid(x) == P.anthropicapi_ResponseContainerUploadBlockType_Valid(x) : Bool}

law anthropicapi_ResponseContentBlockLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseContentBlockLocationCitationType_Valid(x) == P.anthropicapi_ResponseContentBlockLocationCitationType_Valid(x) : Bool}

law anthropicapi_ResponseDocumentBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseDocumentBlockType_Valid(x) == P.anthropicapi_ResponseDocumentBlockType_Valid(x) : Bool}

law anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid(x) == P.anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponsePageLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponsePageLocationCitationType_Valid(x) == P.anthropicapi_ResponsePageLocationCitationType_Valid(x) : Bool}

law anthropicapi_ResponseRedactedThinkingBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseRedactedThinkingBlockType_Valid(x) == P.anthropicapi_ResponseRedactedThinkingBlockType_Valid(x) : Bool}

law anthropicapi_ResponseSearchResultLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseSearchResultLocationCitationType_Valid(x) == P.anthropicapi_ResponseSearchResultLocationCitationType_Valid(x) : Bool}

law anthropicapi_ResponseServerToolUseBlockName_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseServerToolUseBlockName_Valid(x) == P.anthropicapi_ResponseServerToolUseBlockName_Valid(x) : Bool}

law anthropicapi_ResponseServerToolUseBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseServerToolUseBlockType_Valid(x) == P.anthropicapi_ResponseServerToolUseBlockType_Valid(x) : Bool}

law anthropicapi_ResponseTextBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextBlockType_Valid(x) == P.anthropicapi_ResponseTextBlockType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid(x) : Bool}

law anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid(x) == P.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseThinkingBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseThinkingBlockType_Valid(x) == P.anthropicapi_ResponseThinkingBlockType_Valid(x) : Bool}

law anthropicapi_ResponseToolReferenceBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseToolReferenceBlockType_Valid(x) == P.anthropicapi_ResponseToolReferenceBlockType_Valid(x) : Bool}

law anthropicapi_ResponseToolSearchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseToolSearchToolResultBlockType_Valid(x) == P.anthropicapi_ResponseToolSearchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseToolSearchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseToolSearchToolResultErrorType_Valid(x) == P.anthropicapi_ResponseToolSearchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid(x) == P.anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseToolUseBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseToolUseBlockType_Valid(x) == P.anthropicapi_ResponseToolUseBlockType_Valid(x) : Bool}

law anthropicapi_ResponseWebFetchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebFetchResultBlockType_Valid(x) == P.anthropicapi_ResponseWebFetchResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseWebFetchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebFetchToolResultBlockType_Valid(x) == P.anthropicapi_ResponseWebFetchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseWebFetchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebFetchToolResultErrorType_Valid(x) == P.anthropicapi_ResponseWebFetchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ResponseWebSearchResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebSearchResultBlockType_Valid(x) == P.anthropicapi_ResponseWebSearchResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseWebSearchResultLocationCitationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebSearchResultLocationCitationType_Valid(x) == P.anthropicapi_ResponseWebSearchResultLocationCitationType_Valid(x) : Bool}

law anthropicapi_ResponseWebSearchToolResultBlockType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebSearchToolResultBlockType_Valid(x) == P.anthropicapi_ResponseWebSearchToolResultBlockType_Valid(x) : Bool}

law anthropicapi_ResponseWebSearchToolResultErrorType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ResponseWebSearchToolResultErrorType_Valid(x) == P.anthropicapi_ResponseWebSearchToolResultErrorType_Valid(x) : Bool}

law anthropicapi_ServerToolCallerType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ServerToolCallerType_Valid(x) == P.anthropicapi_ServerToolCallerType_Valid(x) : Bool}

law anthropicapi_ServerToolCaller20260120Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ServerToolCaller20260120Type_Valid(x) == P.anthropicapi_ServerToolCaller20260120Type_Valid(x) : Bool}

law anthropicapi_SignatureContentBlockDeltaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_SignatureContentBlockDeltaType_Valid(x) == P.anthropicapi_SignatureContentBlockDeltaType_Valid(x) : Bool}

law anthropicapi_SkillParamsType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_SkillParamsType_Valid(x) == P.anthropicapi_SkillParamsType_Valid(x) : Bool}

law anthropicapi_StopReason_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_StopReason_Valid(x) == P.anthropicapi_StopReason_Valid(x) : Bool}

law anthropicapi_TextContentBlockDeltaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextContentBlockDeltaType_Valid(x) == P.anthropicapi_TextContentBlockDeltaType_Valid(x) : Bool}

law anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid(x) == P.anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_TextEditor20250124Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250124Name_Valid(x) == P.anthropicapi_TextEditor20250124Name_Valid(x) : Bool}

law anthropicapi_TextEditor20250124Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250124Type_Valid(x) == P.anthropicapi_TextEditor20250124Type_Valid(x) : Bool}

law anthropicapi_TextEditor20250429Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250429Name_Valid(x) == P.anthropicapi_TextEditor20250429Name_Valid(x) : Bool}

law anthropicapi_TextEditor20250429Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250429Type_Valid(x) == P.anthropicapi_TextEditor20250429Type_Valid(x) : Bool}

law anthropicapi_TextEditor20250728Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250728Name_Valid(x) == P.anthropicapi_TextEditor20250728Name_Valid(x) : Bool}

law anthropicapi_TextEditor20250728Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_TextEditor20250728Type_Valid(x) == P.anthropicapi_TextEditor20250728Type_Valid(x) : Bool}

law anthropicapi_ThinkingConfigAdaptiveType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ThinkingConfigAdaptiveType_Valid(x) == P.anthropicapi_ThinkingConfigAdaptiveType_Valid(x) : Bool}

law anthropicapi_ThinkingConfigDisabledType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ThinkingConfigDisabledType_Valid(x) == P.anthropicapi_ThinkingConfigDisabledType_Valid(x) : Bool}

law anthropicapi_ThinkingConfigEnabledType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ThinkingConfigEnabledType_Valid(x) == P.anthropicapi_ThinkingConfigEnabledType_Valid(x) : Bool}

law anthropicapi_ThinkingContentBlockDeltaType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ThinkingContentBlockDeltaType_Valid(x) == P.anthropicapi_ThinkingContentBlockDeltaType_Valid(x) : Bool}

law anthropicapi_ThinkingDisplayMode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ThinkingDisplayMode_Valid(x) == P.anthropicapi_ThinkingDisplayMode_Valid(x) : Bool}

law anthropicapi_ToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolType_Valid(x) == P.anthropicapi_ToolType_Valid(x) : Bool}

law anthropicapi_ToolChoiceAnyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolChoiceAnyType_Valid(x) == P.anthropicapi_ToolChoiceAnyType_Valid(x) : Bool}

law anthropicapi_ToolChoiceAutoType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolChoiceAutoType_Valid(x) == P.anthropicapi_ToolChoiceAutoType_Valid(x) : Bool}

law anthropicapi_ToolChoiceNoneType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolChoiceNoneType_Valid(x) == P.anthropicapi_ToolChoiceNoneType_Valid(x) : Bool}

law anthropicapi_ToolChoiceToolType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolChoiceToolType_Valid(x) == P.anthropicapi_ToolChoiceToolType_Valid(x) : Bool}

law anthropicapi_ToolSearchToolBM2520251119Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolSearchToolBM2520251119Name_Valid(x) == P.anthropicapi_ToolSearchToolBM2520251119Name_Valid(x) : Bool}

law anthropicapi_ToolSearchToolBM2520251119Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolSearchToolBM2520251119Type_Valid(x) == P.anthropicapi_ToolSearchToolBM2520251119Type_Valid(x) : Bool}

law anthropicapi_ToolSearchToolRegex20251119Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolSearchToolRegex20251119Name_Valid(x) == P.anthropicapi_ToolSearchToolRegex20251119Name_Valid(x) : Bool}

law anthropicapi_ToolSearchToolRegex20251119Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolSearchToolRegex20251119Type_Valid(x) == P.anthropicapi_ToolSearchToolRegex20251119Type_Valid(x) : Bool}

law anthropicapi_ToolSearchToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_ToolSearchToolResultErrorCode_Valid(x) == P.anthropicapi_ToolSearchToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_URLImageSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_URLImageSourceType_Valid(x) == P.anthropicapi_URLImageSourceType_Valid(x) : Bool}

law anthropicapi_URLPDFSourceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_URLPDFSourceType_Valid(x) == P.anthropicapi_URLPDFSourceType_Valid(x) : Bool}

law anthropicapi_UsageServiceTier_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_UsageServiceTier_Valid(x) == P.anthropicapi_UsageServiceTier_Valid(x) : Bool}

law anthropicapi_UserLocationType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_UserLocationType_Valid(x) == P.anthropicapi_UserLocationType_Valid(x) : Bool}

law anthropicapi_WebFetchToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchToolResultErrorCode_Valid(x) == P.anthropicapi_WebFetchToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_WebFetchTool20250910Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20250910Name_Valid(x) == P.anthropicapi_WebFetchTool20250910Name_Valid(x) : Bool}

law anthropicapi_WebFetchTool20250910Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20250910Type_Valid(x) == P.anthropicapi_WebFetchTool20250910Type_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260209Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260209Name_Valid(x) == P.anthropicapi_WebFetchTool20260209Name_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260209Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260209Type_Valid(x) == P.anthropicapi_WebFetchTool20260209Type_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260309Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260309Name_Valid(x) == P.anthropicapi_WebFetchTool20260309Name_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260309Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260309Type_Valid(x) == P.anthropicapi_WebFetchTool20260309Type_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260318Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260318Name_Valid(x) == P.anthropicapi_WebFetchTool20260318Name_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260318ResponseInclusion_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260318ResponseInclusion_Valid(x) == P.anthropicapi_WebFetchTool20260318ResponseInclusion_Valid(x) : Bool}

law anthropicapi_WebFetchTool20260318Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchTool20260318Type_Valid(x) == P.anthropicapi_WebFetchTool20260318Type_Valid(x) : Bool}

law anthropicapi_WebFetchUrlSourceAllType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchUrlSourceAllType_Valid(x) == P.anthropicapi_WebFetchUrlSourceAllType_Valid(x) : Bool}

law anthropicapi_WebFetchUrlSourceExceptType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchUrlSourceExceptType_Valid(x) == P.anthropicapi_WebFetchUrlSourceExceptType_Valid(x) : Bool}

law anthropicapi_WebFetchUrlSourceNoneType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchUrlSourceNoneType_Valid(x) == P.anthropicapi_WebFetchUrlSourceNoneType_Valid(x) : Bool}

law anthropicapi_WebFetchUrlSourceOnlyType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchUrlSourceOnlyType_Valid(x) == P.anthropicapi_WebFetchUrlSourceOnlyType_Valid(x) : Bool}

law anthropicapi_WebFetchUrlSourceToolReferenceType_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebFetchUrlSourceToolReferenceType_Valid(x) == P.anthropicapi_WebFetchUrlSourceToolReferenceType_Valid(x) : Bool}

law anthropicapi_WebSearchToolResultErrorCode_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchToolResultErrorCode_Valid(x) == P.anthropicapi_WebSearchToolResultErrorCode_Valid(x) : Bool}

law anthropicapi_WebSearchTool20250305Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20250305Name_Valid(x) == P.anthropicapi_WebSearchTool20250305Name_Valid(x) : Bool}

law anthropicapi_WebSearchTool20250305Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20250305Type_Valid(x) == P.anthropicapi_WebSearchTool20250305Type_Valid(x) : Bool}

law anthropicapi_WebSearchTool20260209Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20260209Name_Valid(x) == P.anthropicapi_WebSearchTool20260209Name_Valid(x) : Bool}

law anthropicapi_WebSearchTool20260209Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20260209Type_Valid(x) == P.anthropicapi_WebSearchTool20260209Type_Valid(x) : Bool}

law anthropicapi_WebSearchTool20260318Name_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20260318Name_Valid(x) == P.anthropicapi_WebSearchTool20260318Name_Valid(x) : Bool}

law anthropicapi_WebSearchTool20260318ResponseInclusion_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20260318ResponseInclusion_Valid(x) == P.anthropicapi_WebSearchTool20260318ResponseInclusion_Valid(x) : Bool}

law anthropicapi_WebSearchTool20260318Type_Valid_matches_policy:
  for +x: List<&2, Nat>
  {S.anthropicapi_WebSearchTool20260318Type_Valid(x) == P.anthropicapi_WebSearchTool20260318Type_Valid(x) : Bool}


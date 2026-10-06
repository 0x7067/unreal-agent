import Base
import ../enums/semantics.bend as M
import ./helpers.bend as H
import ./POLICY.bend as P
import ./LAWS.bend as Laws

def Laws.branch_or(b, r):
  match b:
    case True{}: {==}
    case False{}: {==}

def Laws.chain_membership(values, x):
  match values:
    case Nil{}: {==}
    case Con{+value, +rest}:
      Equal.trans(Bool,
        H.chain(value <> rest, x),
        M.choose(M.byteEq(x, value), True{}, H.members(rest, x)),
        H.members(value <> rest, x),
        Equal.cong(Bool, Bool, r => M.choose(M.byteEq(x, value), True{}, r),
          H.chain(rest, x), H.members(rest, x), Laws.chain_membership(rest, x)),
        Laws.branch_or(M.byteEq(x, value), H.members(rest, x)))

def Laws.openaiapi_AdditionalToolsType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_AdditionalToolsType_Valid_constants(), x)

def Laws.openaiapi_AdditionalToolsItemParamRole_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_AdditionalToolsItemParamRole_Valid_constants(), x)

def Laws.openaiapi_AdditionalToolsItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_AdditionalToolsItemParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCallOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCallOutputStatus_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCallOutputStatusParam_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCallOutputStatusParam_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCallStatus_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCallStatusParam_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCallStatusParam_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCreateFileOperationType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCreateFileOperationType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchCreateFileOperationParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchCreateFileOperationParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchDeleteFileOperationType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchDeleteFileOperationType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchDeleteFileOperationParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchDeleteFileOperationParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchToolCallType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchToolCallItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchToolCallItemParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchToolCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchToolCallOutputType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchToolCallOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchToolCallOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchToolParamType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchUpdateFileOperationType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchUpdateFileOperationType_Valid_constants(), x)

def Laws.openaiapi_ApplyPatchUpdateFileOperationParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApplyPatchUpdateFileOperationParamType_Valid_constants(), x)

def Laws.openaiapi_ApproximateLocationType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ApproximateLocationType_Valid_constants(), x)

def Laws.openaiapi_AutoCodeInterpreterToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_AutoCodeInterpreterToolParamType_Valid_constants(), x)

def Laws.openaiapi_CallableToolAllowedCaller_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CallableToolAllowedCaller_Valid_constants(), x)

def Laws.openaiapi_ClickButtonType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ClickButtonType_Valid_constants(), x)

def Laws.openaiapi_ClickParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ClickParamType_Valid_constants(), x)

def Laws.openaiapi_CodeInterpreterOutputImageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CodeInterpreterOutputImageType_Valid_constants(), x)

def Laws.openaiapi_CodeInterpreterOutputLogsType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CodeInterpreterOutputLogsType_Valid_constants(), x)

def Laws.openaiapi_CodeInterpreterToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CodeInterpreterToolType_Valid_constants(), x)

def Laws.openaiapi_CodeInterpreterToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CodeInterpreterToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_CodeInterpreterToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CodeInterpreterToolCallType_Valid_constants(), x)

def Laws.openaiapi_CompactionBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CompactionBodyType_Valid_constants(), x)

def Laws.openaiapi_CompactionSummaryItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CompactionSummaryItemParamType_Valid_constants(), x)

def Laws.openaiapi_CompactionTriggerItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CompactionTriggerItemParamType_Valid_constants(), x)

def Laws.openaiapi_ComparisonFilterType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComparisonFilterType_Valid_constants(), x)

def Laws.openaiapi_CompoundFilterType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CompoundFilterType_Valid_constants(), x)

def Laws.openaiapi_ComputerCallOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerCallOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_ComputerCallOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerCallOutputStatus_Valid_constants(), x)

def Laws.openaiapi_ComputerEnvironment_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerEnvironment_Valid_constants(), x)

def Laws.openaiapi_ComputerScreenshotImageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerScreenshotImageType_Valid_constants(), x)

def Laws.openaiapi_ComputerToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolType_Valid_constants(), x)

def Laws.openaiapi_ComputerToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_ComputerToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolCallType_Valid_constants(), x)

def Laws.openaiapi_ComputerToolCallOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolCallOutputStatus_Valid_constants(), x)

def Laws.openaiapi_ComputerToolCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolCallOutputType_Valid_constants(), x)

def Laws.openaiapi_ComputerToolCallOutputResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerToolCallOutputResourceType_Valid_constants(), x)

def Laws.openaiapi_ComputerUsePreviewToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ComputerUsePreviewToolType_Valid_constants(), x)

def Laws.openaiapi_ContainerAutoParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerAutoParamType_Valid_constants(), x)

def Laws.openaiapi_ContainerFileCitationBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerFileCitationBodyType_Valid_constants(), x)

def Laws.openaiapi_ContainerMemoryLimit_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerMemoryLimit_Valid_constants(), x)

def Laws.openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerNetworkPolicyAllowlistParamType_Valid_constants(), x)

def Laws.openaiapi_ContainerNetworkPolicyDisabledParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerNetworkPolicyDisabledParamType_Valid_constants(), x)

def Laws.openaiapi_ContainerReferenceParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerReferenceParamType_Valid_constants(), x)

def Laws.openaiapi_ContainerReferenceResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ContainerReferenceResourceType_Valid_constants(), x)

def Laws.openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CreateModelResponsePropertiesPromptCacheRetention_Valid_constants(), x)

def Laws.openaiapi_CreateResponsePromptCacheRetention_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CreateResponsePromptCacheRetention_Valid_constants(), x)

def Laws.openaiapi_CreateResponseTruncation_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CreateResponseTruncation_Valid_constants(), x)

def Laws.openaiapi_CustomGrammarFormatParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomGrammarFormatParamType_Valid_constants(), x)

def Laws.openaiapi_CustomTextFormatParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomTextFormatParamType_Valid_constants(), x)

def Laws.openaiapi_CustomToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomToolCallType_Valid_constants(), x)

def Laws.openaiapi_CustomToolCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomToolCallOutputType_Valid_constants(), x)

def Laws.openaiapi_CustomToolCallOutputResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomToolCallOutputResourceType_Valid_constants(), x)

def Laws.openaiapi_CustomToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_CustomToolParamType_Valid_constants(), x)

def Laws.openaiapi_DetailEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_DetailEnum_Valid_constants(), x)

def Laws.openaiapi_DirectToolCallCallerType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_DirectToolCallCallerType_Valid_constants(), x)

def Laws.openaiapi_DirectToolCallCallerParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_DirectToolCallCallerParamType_Valid_constants(), x)

def Laws.openaiapi_DoubleClickActionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_DoubleClickActionType_Valid_constants(), x)

def Laws.openaiapi_DragParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_DragParamType_Valid_constants(), x)

def Laws.openaiapi_EasyInputMessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_EasyInputMessageRole_Valid_constants(), x)

def Laws.openaiapi_EasyInputMessageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_EasyInputMessageType_Valid_constants(), x)

def Laws.openaiapi_FileCitationBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileCitationBodyType_Valid_constants(), x)

def Laws.openaiapi_FileDetailEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileDetailEnum_Valid_constants(), x)

def Laws.openaiapi_FileInputDetail_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileInputDetail_Valid_constants(), x)

def Laws.openaiapi_FilePathType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FilePathType_Valid_constants(), x)

def Laws.openaiapi_FileSearchToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileSearchToolType_Valid_constants(), x)

def Laws.openaiapi_FileSearchToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileSearchToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_FileSearchToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FileSearchToolCallType_Valid_constants(), x)

def Laws.openaiapi_FunctionCallItemStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionCallItemStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionCallOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionCallOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionCallOutputStatusEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionCallOutputStatusEnum_Valid_constants(), x)

def Laws.openaiapi_FunctionCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionCallStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallItemParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallItemStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallItemStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputExitOutcomeType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputExitOutcomeType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputExitOutcomeParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputStatusEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputStatusEnum_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputTimeoutOutcomeType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallOutputTimeoutOutcomeParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionShellCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellCallStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionShellToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionShellToolParamType_Valid_constants(), x)

def Laws.openaiapi_FunctionToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolType_Valid_constants(), x)

def Laws.openaiapi_FunctionToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolCallType_Valid_constants(), x)

def Laws.openaiapi_FunctionToolCallOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolCallOutputStatus_Valid_constants(), x)

def Laws.openaiapi_FunctionToolCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolCallOutputType_Valid_constants(), x)

def Laws.openaiapi_FunctionToolCallOutputResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolCallOutputResourceType_Valid_constants(), x)

def Laws.openaiapi_FunctionToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_FunctionToolParamType_Valid_constants(), x)

def Laws.openaiapi_GrammarSyntax1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_GrammarSyntax1_Valid_constants(), x)

def Laws.openaiapi_ImageDetail_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageDetail_Valid_constants(), x)

def Laws.openaiapi_ImageGenActionEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenActionEnum_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolBackground_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolBackground_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolModel1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolModel1_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolModeration_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolModeration_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolOutputFormat_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolOutputFormat_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolQuality_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolQuality_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolSize1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolSize1_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolType_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_ImageGenToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ImageGenToolCallType_Valid_constants(), x)

def Laws.openaiapi_IncludeEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_IncludeEnum_Valid_constants(), x)

def Laws.openaiapi_InlineSkillParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InlineSkillParamType_Valid_constants(), x)

def Laws.openaiapi_InlineSkillSourceParamMediaType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InlineSkillSourceParamMediaType_Valid_constants(), x)

def Laws.openaiapi_InlineSkillSourceParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InlineSkillSourceParamType_Valid_constants(), x)

def Laws.openaiapi_InputFidelity_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputFidelity_Valid_constants(), x)

def Laws.openaiapi_InputFileContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputFileContentType_Valid_constants(), x)

def Laws.openaiapi_InputFileContentParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputFileContentParamType_Valid_constants(), x)

def Laws.openaiapi_InputImageContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputImageContentType_Valid_constants(), x)

def Laws.openaiapi_InputImageContentParamAutoParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputImageContentParamAutoParamType_Valid_constants(), x)

def Laws.openaiapi_InputMessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputMessageRole_Valid_constants(), x)

def Laws.openaiapi_InputMessageStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputMessageStatus_Valid_constants(), x)

def Laws.openaiapi_InputMessageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputMessageType_Valid_constants(), x)

def Laws.openaiapi_InputTextContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputTextContentType_Valid_constants(), x)

def Laws.openaiapi_InputTextContentParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_InputTextContentParamType_Valid_constants(), x)

def Laws.openaiapi_ItemReferenceParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ItemReferenceParamType_Valid_constants(), x)

def Laws.openaiapi_KeyPressActionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_KeyPressActionType_Valid_constants(), x)

def Laws.openaiapi_LocalEnvironmentParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalEnvironmentParamType_Valid_constants(), x)

def Laws.openaiapi_LocalEnvironmentResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalEnvironmentResourceType_Valid_constants(), x)

def Laws.openaiapi_LocalShellExecActionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellExecActionType_Valid_constants(), x)

def Laws.openaiapi_LocalShellToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_LocalShellToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellToolCallType_Valid_constants(), x)

def Laws.openaiapi_LocalShellToolCallOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellToolCallOutputStatus_Valid_constants(), x)

def Laws.openaiapi_LocalShellToolCallOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellToolCallOutputType_Valid_constants(), x)

def Laws.openaiapi_LocalShellToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_LocalShellToolParamType_Valid_constants(), x)

def Laws.openaiapi_MCPApprovalRequestType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPApprovalRequestType_Valid_constants(), x)

def Laws.openaiapi_MCPApprovalResponseType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPApprovalResponseType_Valid_constants(), x)

def Laws.openaiapi_MCPApprovalResponseResourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPApprovalResponseResourceType_Valid_constants(), x)

def Laws.openaiapi_MCPListToolsType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPListToolsType_Valid_constants(), x)

def Laws.openaiapi_MCPToolConnectorId_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPToolConnectorId_Valid_constants(), x)

def Laws.openaiapi_MCPToolRequireApproval1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPToolRequireApproval1_Valid_constants(), x)

def Laws.openaiapi_MCPToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPToolType_Valid_constants(), x)

def Laws.openaiapi_MCPToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPToolCallType_Valid_constants(), x)

def Laws.openaiapi_MCPToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MCPToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_MessagePhase_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MessagePhase_Valid_constants(), x)

def Laws.openaiapi_MessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MessageRole_Valid_constants(), x)

def Laws.openaiapi_ModelIdsResponses1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModelIdsResponses1_Valid_constants(), x)

def Laws.openaiapi_ModelIdsShared1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModelIdsShared1_Valid_constants(), x)

def Laws.openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModelResponsePropertiesPromptCacheRetention_Valid_constants(), x)

def Laws.openaiapi_ModerationErrorBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModerationErrorBodyType_Valid_constants(), x)

def Laws.openaiapi_ModerationInputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModerationInputType_Valid_constants(), x)

def Laws.openaiapi_ModerationMode_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModerationMode_Valid_constants(), x)

def Laws.openaiapi_ModerationResultBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ModerationResultBodyType_Valid_constants(), x)

def Laws.openaiapi_MoveParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_MoveParamType_Valid_constants(), x)

def Laws.openaiapi_NamespaceToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_NamespaceToolParamType_Valid_constants(), x)

def Laws.openaiapi_OutputMessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_OutputMessageRole_Valid_constants(), x)

def Laws.openaiapi_OutputMessageStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_OutputMessageStatus_Valid_constants(), x)

def Laws.openaiapi_OutputMessageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_OutputMessageType_Valid_constants(), x)

def Laws.openaiapi_OutputTextContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_OutputTextContentType_Valid_constants(), x)

def Laws.openaiapi_ProgramType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramType_Valid_constants(), x)

def Laws.openaiapi_ProgramItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramItemParamType_Valid_constants(), x)

def Laws.openaiapi_ProgramOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramOutputType_Valid_constants(), x)

def Laws.openaiapi_ProgramOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_ProgramOutputItemStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramOutputItemStatus_Valid_constants(), x)

def Laws.openaiapi_ProgramOutputStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramOutputStatus_Valid_constants(), x)

def Laws.openaiapi_ProgramToolCallCallerType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramToolCallCallerType_Valid_constants(), x)

def Laws.openaiapi_ProgramToolCallCallerParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgramToolCallCallerParamType_Valid_constants(), x)

def Laws.openaiapi_ProgrammaticToolCallingParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ProgrammaticToolCallingParamType_Valid_constants(), x)

def Laws.openaiapi_PromptCacheBreakpointConfigMode_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_PromptCacheBreakpointConfigMode_Valid_constants(), x)

def Laws.openaiapi_PromptCacheBreakpointParamMode_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_PromptCacheBreakpointParamMode_Valid_constants(), x)

def Laws.openaiapi_PromptCacheModeEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_PromptCacheModeEnum_Valid_constants(), x)

def Laws.openaiapi_PromptCacheTTLEnum_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_PromptCacheTTLEnum_Valid_constants(), x)

def Laws.openaiapi_RankerVersionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_RankerVersionType_Valid_constants(), x)

def Laws.openaiapi_ReasoningContext_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningContext_Valid_constants(), x)

def Laws.openaiapi_ReasoningGenerateSummary_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningGenerateSummary_Valid_constants(), x)

def Laws.openaiapi_ReasoningSummary_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningSummary_Valid_constants(), x)

def Laws.openaiapi_ReasoningItemStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningItemStatus_Valid_constants(), x)

def Laws.openaiapi_ReasoningItemType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningItemType_Valid_constants(), x)

def Laws.openaiapi_ReasoningModeEnum1_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningModeEnum1_Valid_constants(), x)

def Laws.openaiapi_ReasoningTextContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ReasoningTextContentType_Valid_constants(), x)

def Laws.openaiapi_RefusalContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_RefusalContentType_Valid_constants(), x)

def Laws.openaiapi_ResponseIncompleteDetailsReason_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseIncompleteDetailsReason_Valid_constants(), x)

def Laws.openaiapi_ResponseObject_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseObject_Valid_constants(), x)

def Laws.openaiapi_ResponsePromptCacheRetention_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponsePromptCacheRetention_Valid_constants(), x)

def Laws.openaiapi_ResponseStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseStatus_Valid_constants(), x)

def Laws.openaiapi_ResponseTruncation_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseTruncation_Valid_constants(), x)

def Laws.openaiapi_ResponseAudioDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseAudioDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseAudioDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseAudioDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseAudioTranscriptDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseAudioTranscriptDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseAudioTranscriptDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseAudioTranscriptDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCodeInterpreterCallCodeDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCodeInterpreterCallCodeDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCodeInterpreterCallCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCodeInterpreterCallInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCodeInterpreterCallInterpretingEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseContentPartAddedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseContentPartAddedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseContentPartDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseContentPartDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCreatedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCreatedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCustomToolCallInputDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseCustomToolCallInputDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseCustomToolCallInputDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseErrorCode_Valid_constants(), x)

def Laws.openaiapi_ResponseErrorEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseErrorEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFailedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFailedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFileSearchCallCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFileSearchCallCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFileSearchCallInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFileSearchCallInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFileSearchCallSearchingEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFileSearchCallSearchingEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFormatJsonObjectType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFormatJsonObjectType_Valid_constants(), x)

def Laws.openaiapi_ResponseFormatTextType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFormatTextType_Valid_constants(), x)

def Laws.openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFunctionCallArgumentsDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseFunctionCallArgumentsDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseImageGenCallCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseImageGenCallCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseImageGenCallGeneratingEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseImageGenCallGeneratingEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseImageGenCallInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseImageGenCallInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseImageGenCallPartialImageEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseImageGenCallPartialImageEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseIncompleteEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseIncompleteEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPCallArgumentsDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPCallArgumentsDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPCallCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPCallCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPCallFailedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPCallFailedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPCallInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPCallInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPListToolsCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPListToolsCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPListToolsFailedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPListToolsFailedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseMCPListToolsInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseMCPListToolsInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseOutputItemAddedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseOutputItemAddedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseOutputItemDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseOutputItemDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseOutputTextAnnotationAddedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseQueuedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseQueuedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryPartAddedEventPartType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryPartAddedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryPartDoneEventPartType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryPartDoneEventStatus_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryPartDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryTextDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningSummaryTextDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningTextDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningTextDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseReasoningTextDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseReasoningTextDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseRefusalDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseRefusalDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseRefusalDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseRefusalDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseTextDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseTextDeltaEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseTextDoneEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseTextDoneEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseWebSearchCallCompletedEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseWebSearchCallCompletedEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseWebSearchCallInProgressEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseWebSearchCallInProgressEventType_Valid_constants(), x)

def Laws.openaiapi_ResponseWebSearchCallSearchingEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ResponseWebSearchCallSearchingEventType_Valid_constants(), x)

def Laws.openaiapi_ScreenshotParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ScreenshotParamType_Valid_constants(), x)

def Laws.openaiapi_ScrollParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ScrollParamType_Valid_constants(), x)

def Laws.openaiapi_SearchContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SearchContentType_Valid_constants(), x)

def Laws.openaiapi_SearchContextSize_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SearchContextSize_Valid_constants(), x)

def Laws.openaiapi_SkillReferenceParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SkillReferenceParamType_Valid_constants(), x)

def Laws.openaiapi_SpecificApplyPatchParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SpecificApplyPatchParamType_Valid_constants(), x)

def Laws.openaiapi_SpecificFunctionShellParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SpecificFunctionShellParamType_Valid_constants(), x)

def Laws.openaiapi_SpecificProgrammaticToolCallingParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SpecificProgrammaticToolCallingParamType_Valid_constants(), x)

def Laws.openaiapi_SummaryTextContentType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_SummaryTextContentType_Valid_constants(), x)

def Laws.openaiapi_TextResponseFormatJsonSchemaType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_TextResponseFormatJsonSchemaType_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceAllowedMode_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceAllowedMode_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceAllowedType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceAllowedType_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceCustomType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceCustomType_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceFunctionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceFunctionType_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceMCPType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceMCPType_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceOptions_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceOptions_Valid_constants(), x)

def Laws.openaiapi_ToolChoiceTypesType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolChoiceTypesType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchCallType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchCallItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchCallItemParamType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchExecutionType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchExecutionType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchOutputType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchOutputType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchOutputItemParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchOutputItemParamType_Valid_constants(), x)

def Laws.openaiapi_ToolSearchToolParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_ToolSearchToolParamType_Valid_constants(), x)

def Laws.openaiapi_TypeParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_TypeParamType_Valid_constants(), x)

def Laws.openaiapi_UrlCitationBodyType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_UrlCitationBodyType_Valid_constants(), x)

def Laws.openaiapi_WaitParamType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WaitParamType_Valid_constants(), x)

def Laws.openaiapi_WebSearchActionFindType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchActionFindType_Valid_constants(), x)

def Laws.openaiapi_WebSearchActionOpenPageType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchActionOpenPageType_Valid_constants(), x)

def Laws.openaiapi_WebSearchActionSearchSourcesType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchActionSearchSourcesType_Valid_constants(), x)

def Laws.openaiapi_WebSearchActionSearchType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchActionSearchType_Valid_constants(), x)

def Laws.openaiapi_WebSearchApproximateLocationType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchApproximateLocationType_Valid_constants(), x)

def Laws.openaiapi_WebSearchPreviewToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchPreviewToolType_Valid_constants(), x)

def Laws.openaiapi_WebSearchToolSearchContextSize_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchToolSearchContextSize_Valid_constants(), x)

def Laws.openaiapi_WebSearchToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchToolType_Valid_constants(), x)

def Laws.openaiapi_WebSearchToolCallStatus_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchToolCallStatus_Valid_constants(), x)

def Laws.openaiapi_WebSearchToolCallType_Valid_matches_policy(x):
  Laws.chain_membership(P.openaiapi_WebSearchToolCallType_Valid_constants(), x)

def Laws.anthropicapi_APIErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_APIErrorType_Valid_constants(), x)

def Laws.anthropicapi_AllowedCaller_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_AllowedCaller_Valid_constants(), x)

def Laws.anthropicapi_AuthenticationErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_AuthenticationErrorType_Valid_constants(), x)

def Laws.anthropicapi_Base64ImageSourceMediaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_Base64ImageSourceMediaType_Valid_constants(), x)

def Laws.anthropicapi_Base64ImageSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_Base64ImageSourceType_Valid_constants(), x)

def Laws.anthropicapi_Base64PDFSourceMediaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_Base64PDFSourceMediaType_Valid_constants(), x)

def Laws.anthropicapi_Base64PDFSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_Base64PDFSourceType_Valid_constants(), x)

def Laws.anthropicapi_BashCodeExecutionToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BashCodeExecutionToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_BashTool20250124Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BashTool20250124Name_Valid_constants(), x)

def Laws.anthropicapi_BashTool20250124Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BashTool20250124Type_Valid_constants(), x)

def Laws.anthropicapi_BillingErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BillingErrorType_Valid_constants(), x)

def Laws.anthropicapi_BrowserStateChangeDownloadCompletedType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BrowserStateChangeDownloadCompletedType_Valid_constants(), x)

def Laws.anthropicapi_BrowserStateChangeDownloadFailedType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BrowserStateChangeDownloadFailedType_Valid_constants(), x)

def Laws.anthropicapi_BrowserStateChangeDownloadStartedType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BrowserStateChangeDownloadStartedType_Valid_constants(), x)

def Laws.anthropicapi_BrowserStateChangeTabOpenedType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BrowserStateChangeTabOpenedType_Valid_constants(), x)

def Laws.anthropicapi_BrowserToolset20260801Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_BrowserToolset20260801Type_Valid_constants(), x)

def Laws.anthropicapi_CacheControlEphemeralTtl_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CacheControlEphemeralTtl_Valid_constants(), x)

def Laws.anthropicapi_CacheControlEphemeralType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CacheControlEphemeralType_Valid_constants(), x)

def Laws.anthropicapi_CitationsDeltaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CitationsDeltaType_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20250522Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20250522Name_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20250522Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20250522Type_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20250825Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20250825Name_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20250825Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20250825Type_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20260120Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20260120Name_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20260120Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20260120Type_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20260521Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20260521Name_Valid_constants(), x)

def Laws.anthropicapi_CodeExecutionTool20260521Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CodeExecutionTool20260521Type_Valid_constants(), x)

def Laws.anthropicapi_ComputerToolset20260801Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ComputerToolset20260801Type_Valid_constants(), x)

def Laws.anthropicapi_ContainerSkillType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ContainerSkillType_Valid_constants(), x)

def Laws.anthropicapi_ContentBlockDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ContentBlockDeltaEventType_Valid_constants(), x)

def Laws.anthropicapi_ContentBlockSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ContentBlockSourceType_Valid_constants(), x)

def Laws.anthropicapi_ContentBlockStartEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ContentBlockStartEventType_Valid_constants(), x)

def Laws.anthropicapi_ContentBlockStopEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ContentBlockStopEventType_Valid_constants(), x)

def Laws.anthropicapi_CreateMessageParamsServiceTier_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_CreateMessageParamsServiceTier_Valid_constants(), x)

def Laws.anthropicapi_DirectCallerType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_DirectCallerType_Valid_constants(), x)

def Laws.anthropicapi_EffortLevel_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_EffortLevel_Valid_constants(), x)

def Laws.anthropicapi_ErrorResponseType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ErrorResponseType_Valid_constants(), x)

def Laws.anthropicapi_FileDocumentSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_FileDocumentSourceType_Valid_constants(), x)

def Laws.anthropicapi_FileImageSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_FileImageSourceType_Valid_constants(), x)

def Laws.anthropicapi_GatewayTimeoutErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_GatewayTimeoutErrorType_Valid_constants(), x)

def Laws.anthropicapi_InputJsonContentBlockDeltaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_InputJsonContentBlockDeltaType_Valid_constants(), x)

def Laws.anthropicapi_InputMessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_InputMessageRole_Valid_constants(), x)

def Laws.anthropicapi_InputSchemaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_InputSchemaType_Valid_constants(), x)

def Laws.anthropicapi_InvalidRequestErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_InvalidRequestErrorType_Valid_constants(), x)

def Laws.anthropicapi_JsonOutputFormatType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_JsonOutputFormatType_Valid_constants(), x)

def Laws.anthropicapi_MemoryTool20250818Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MemoryTool20250818Name_Valid_constants(), x)

def Laws.anthropicapi_MemoryTool20250818Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MemoryTool20250818Type_Valid_constants(), x)

def Laws.anthropicapi_MessageRole_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MessageRole_Valid_constants(), x)

def Laws.anthropicapi_MessageType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MessageType_Valid_constants(), x)

def Laws.anthropicapi_MessageDeltaEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MessageDeltaEventType_Valid_constants(), x)

def Laws.anthropicapi_MessageStartEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MessageStartEventType_Valid_constants(), x)

def Laws.anthropicapi_MessageStopEventType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_MessageStopEventType_Valid_constants(), x)

def Laws.anthropicapi_NotFoundErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_NotFoundErrorType_Valid_constants(), x)

def Laws.anthropicapi_OverloadedErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_OverloadedErrorType_Valid_constants(), x)

def Laws.anthropicapi_PermissionErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_PermissionErrorType_Valid_constants(), x)

def Laws.anthropicapi_PlainTextSourceMediaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_PlainTextSourceMediaType_Valid_constants(), x)

def Laws.anthropicapi_PlainTextSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_PlainTextSourceType_Valid_constants(), x)

def Laws.anthropicapi_RateLimitErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RateLimitErrorType_Valid_constants(), x)

def Laws.anthropicapi_RefusalCategory_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RefusalCategory_Valid_constants(), x)

def Laws.anthropicapi_RefusalStopDetailsType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RefusalStopDetailsType_Valid_constants(), x)

def Laws.anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestBashCodeExecutionOutputBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestBashCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestBashCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestBashCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestBashCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_RequestBrowserStateBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestBrowserStateBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestCharLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestCharLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_RequestCodeExecutionOutputBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestCodeExecutionOutputBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_RequestContainerUploadBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestContainerUploadBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestContentBlockLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestContentBlockLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_RequestDocumentBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestDocumentBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestEncryptedCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestImageBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestImageBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestImageTransformationsOversizedImage_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestImageTransformationsOversizedImage_Valid_constants(), x)

def Laws.anthropicapi_RequestPageLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestPageLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_RequestRedactedThinkingBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestRedactedThinkingBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestSearchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestSearchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestSearchResultLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestSearchResultLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_RequestServerToolUseBlockName_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestServerToolUseBlockName_Valid_constants(), x)

def Laws.anthropicapi_RequestServerToolUseBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestServerToolUseBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionCreateResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockFileType_Valid_constants(), x)

def Laws.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestTextEditorCodeExecutionViewResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestThinkingBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestThinkingBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolReferenceBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolReferenceBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolSearchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolSearchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolSearchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolSearchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolSearchToolSearchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestToolUseBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestToolUseBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebFetchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebFetchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebFetchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebFetchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebFetchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebFetchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebSearchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebSearchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebSearchResultLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebSearchResultLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebSearchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebSearchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_RequestWebSearchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_RequestWebSearchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseBashCodeExecutionOutputBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseBashCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseBashCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseBashCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseCharLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseCharLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_ResponseCodeExecutionOutputBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseCodeExecutionOutputBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseContainerUploadBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseContainerUploadBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseContentBlockLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseContentBlockLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_ResponseDocumentBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseDocumentBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseEncryptedCodeExecutionResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponsePageLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponsePageLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_ResponseRedactedThinkingBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseRedactedThinkingBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseSearchResultLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseSearchResultLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_ResponseServerToolUseBlockName_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseServerToolUseBlockName_Valid_constants(), x)

def Laws.anthropicapi_ResponseServerToolUseBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseServerToolUseBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionCreateResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionStrReplaceResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockFileType_Valid_constants(), x)

def Laws.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseTextEditorCodeExecutionViewResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseThinkingBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseThinkingBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseToolReferenceBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseToolReferenceBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseToolSearchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseToolSearchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseToolSearchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseToolSearchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseToolSearchToolSearchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseToolUseBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseToolUseBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebFetchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebFetchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebFetchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebFetchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebFetchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebFetchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebSearchResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebSearchResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebSearchResultLocationCitationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebSearchResultLocationCitationType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebSearchToolResultBlockType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebSearchToolResultBlockType_Valid_constants(), x)

def Laws.anthropicapi_ResponseWebSearchToolResultErrorType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ResponseWebSearchToolResultErrorType_Valid_constants(), x)

def Laws.anthropicapi_ServerToolCallerType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ServerToolCallerType_Valid_constants(), x)

def Laws.anthropicapi_ServerToolCaller20260120Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ServerToolCaller20260120Type_Valid_constants(), x)

def Laws.anthropicapi_SignatureContentBlockDeltaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_SignatureContentBlockDeltaType_Valid_constants(), x)

def Laws.anthropicapi_SkillParamsType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_SkillParamsType_Valid_constants(), x)

def Laws.anthropicapi_StopReason_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_StopReason_Valid_constants(), x)

def Laws.anthropicapi_TextContentBlockDeltaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextContentBlockDeltaType_Valid_constants(), x)

def Laws.anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditorCodeExecutionToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250124Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250124Name_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250124Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250124Type_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250429Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250429Name_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250429Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250429Type_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250728Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250728Name_Valid_constants(), x)

def Laws.anthropicapi_TextEditor20250728Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_TextEditor20250728Type_Valid_constants(), x)

def Laws.anthropicapi_ThinkingConfigAdaptiveType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ThinkingConfigAdaptiveType_Valid_constants(), x)

def Laws.anthropicapi_ThinkingConfigDisabledType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ThinkingConfigDisabledType_Valid_constants(), x)

def Laws.anthropicapi_ThinkingConfigEnabledType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ThinkingConfigEnabledType_Valid_constants(), x)

def Laws.anthropicapi_ThinkingContentBlockDeltaType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ThinkingContentBlockDeltaType_Valid_constants(), x)

def Laws.anthropicapi_ThinkingDisplayMode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ThinkingDisplayMode_Valid_constants(), x)

def Laws.anthropicapi_ToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolType_Valid_constants(), x)

def Laws.anthropicapi_ToolChoiceAnyType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolChoiceAnyType_Valid_constants(), x)

def Laws.anthropicapi_ToolChoiceAutoType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolChoiceAutoType_Valid_constants(), x)

def Laws.anthropicapi_ToolChoiceNoneType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolChoiceNoneType_Valid_constants(), x)

def Laws.anthropicapi_ToolChoiceToolType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolChoiceToolType_Valid_constants(), x)

def Laws.anthropicapi_ToolSearchToolBM2520251119Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolSearchToolBM2520251119Name_Valid_constants(), x)

def Laws.anthropicapi_ToolSearchToolBM2520251119Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolSearchToolBM2520251119Type_Valid_constants(), x)

def Laws.anthropicapi_ToolSearchToolRegex20251119Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolSearchToolRegex20251119Name_Valid_constants(), x)

def Laws.anthropicapi_ToolSearchToolRegex20251119Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolSearchToolRegex20251119Type_Valid_constants(), x)

def Laws.anthropicapi_ToolSearchToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_ToolSearchToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_URLImageSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_URLImageSourceType_Valid_constants(), x)

def Laws.anthropicapi_URLPDFSourceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_URLPDFSourceType_Valid_constants(), x)

def Laws.anthropicapi_UsageServiceTier_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_UsageServiceTier_Valid_constants(), x)

def Laws.anthropicapi_UserLocationType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_UserLocationType_Valid_constants(), x)

def Laws.anthropicapi_WebFetchToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20250910Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20250910Name_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20250910Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20250910Type_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260209Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260209Name_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260209Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260209Type_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260309Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260309Name_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260309Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260309Type_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260318Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260318Name_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260318ResponseInclusion_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260318ResponseInclusion_Valid_constants(), x)

def Laws.anthropicapi_WebFetchTool20260318Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchTool20260318Type_Valid_constants(), x)

def Laws.anthropicapi_WebFetchUrlSourceAllType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchUrlSourceAllType_Valid_constants(), x)

def Laws.anthropicapi_WebFetchUrlSourceExceptType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchUrlSourceExceptType_Valid_constants(), x)

def Laws.anthropicapi_WebFetchUrlSourceNoneType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchUrlSourceNoneType_Valid_constants(), x)

def Laws.anthropicapi_WebFetchUrlSourceOnlyType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchUrlSourceOnlyType_Valid_constants(), x)

def Laws.anthropicapi_WebFetchUrlSourceToolReferenceType_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebFetchUrlSourceToolReferenceType_Valid_constants(), x)

def Laws.anthropicapi_WebSearchToolResultErrorCode_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchToolResultErrorCode_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20250305Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20250305Name_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20250305Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20250305Type_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20260209Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20260209Name_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20260209Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20260209Type_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20260318Name_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20260318Name_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20260318ResponseInclusion_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20260318ResponseInclusion_Valid_constants(), x)

def Laws.anthropicapi_WebSearchTool20260318Type_Valid_matches_policy(x):
  Laws.chain_membership(P.anthropicapi_WebSearchTool20260318Type_Valid_constants(), x)


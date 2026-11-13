export const UPLOAD_DISABLED_HINT = "未配置存储 Provider，上传已禁用。";

export function getUploadGate(providerId?: string) {
  const uploadEnabled = Boolean(providerId);
  return {
    uploadEnabled,
    helperText: uploadEnabled ? null : UPLOAD_DISABLED_HINT,
  };
}

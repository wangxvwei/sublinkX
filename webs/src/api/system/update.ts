import request from "@/utils/request";

export interface UpdateInfo {
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  releaseUrl: string;
  dockerImage: string;
  updateCommand: string;
  message: string;
}

export function checkUpdate() {
  return request({
    url: "/api/v1/update/check",
    method: "get",
  });
}

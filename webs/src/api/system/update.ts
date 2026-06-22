import request from "@/utils/request";

export interface UpdateInfo {
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  releaseUrl: string;
  dockerImage: string;
  currentImageDigest: string;
  latestImageDigest: string;
  updateCommand: string;
  autoUpdate: boolean;
  autoUpdateMessage: string;
  containerName: string;
  message: string;
}

export interface UpdateStatus {
  status: "idle" | "running" | "success" | "rolled_back" | "failed";
  message: string;
  error?: string;
  startedAt?: string;
  finishedAt?: string;
  targetImage?: string;
  previousImage?: string;
}

export function checkUpdate() {
  return request({
    url: "/api/v1/update/check",
    method: "get",
  });
}

export function getUpdateStatus() {
  return request({
    url: "/api/v1/update/status",
    method: "get",
  });
}

export function applyUpdate() {
  return request({
    url: "/api/v1/update/apply",
    method: "post",
  });
}

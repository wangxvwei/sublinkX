import request from "@/utils/request";

export interface CDNResult {
  ip: string; delay: number; loss: number; speed: number; colo: string;
  tlsOK: boolean; link: string; name: string;
  recommendation?: boolean;
  pinned?: boolean; sources?: string[]; sourceUpdatedAt?: string; historySamples?: number; basis?: string;
}
export interface CDNFeedStatus { id: string; name: string; url: string; count: number; updatedAt?: string; error?: string }
export interface CDNProfile {
  id: number; name: string; templateLink: string; ips: string; count: number;
  candidateMode?: "recommend" | "manual"; sourceIP?: string; carrier?: string;
  network?: { location: string; province: string; carrier: string; source: string; sourceUpdatedAt?: string; warning: string; sources?: CDNFeedStatus[]; candidateCount?: number; historyWindows?: number };
  url: string; maxDelay: number; maxLoss: number; minSpeed: number; duration: number;
  groupName: string; status: string; message: string; results: CDNResult[] | null;
  pinnedIPs?: string[] | null;
  startedAt?: string; finishedAt?: string; updatedAt?: string;
}
const base = "/api/v1/nodes/cdn";
export const getCDN = () => request({ url: base });
export const saveCDN = (data: CDNProfile, rename = false) => request({ url: `${base}/save`, method: "post", data: { ...data, rename } });
export const runCDN = (id: number) => request({ url: `${base}/${id}/run`, method: "post" });
export const cancelCDN = (id: number) => request({ url: `${base}/${id}/cancel`, method: "post" });
export const importCDN = (id: number, data: { ips: string[]; subscriptionID: number }) =>
  request({ url: `${base}/${id}/import`, method: "post", data });
export const setCDNPinnedIP = (id: number, ip: string, keep: boolean) =>
  request({ url: `${base}/${id}/pinned`, method: "post", data: { ip, keep } });

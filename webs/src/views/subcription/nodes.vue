<script setup lang='ts'>
import { ref,onMounted,nextTick  } from 'vue'
import {getNodes,AddNodes,DelNode,UpdateNode,GetGroup,SetGroup,SyncXUINodes,GetXUISources,SaveXUISource,DelXUISource,SyncXUISource,SyncAllXUISources} from "@/api/subcription/node"

interface GroupNode {
  ID: number;
  Name: string;
  Nodes :Node[];
}
interface Node {
  ID: number;
  Name: string;
  Link: string;
  CreateDate: string;
  GroupNodes?: GroupNode[]; // 鍒嗙粍淇℃伅
  
}
interface NodeInfo {
    ID?: number // 缂栬緫鏃堕渶瑕佷紶鍏D
    Title?:string 
    Name?: string
    Link: string
    GroupName?: string[] // 鍒嗙粍鍚嶇О
}
interface XUISource {
  id?: number
  name: string
  host: string
  sshPort: number
  username: string
  authType?: 'password' | 'apiToken'
  password?: string
  panelBaseUrl?: string
  apiToken?: string
  xuiDbPath: string
  subBaseUrl: string
  subPath: string
  groupName: string
  namePrefix: string
  rewriteRules: string
  deleteMissing: boolean
  enabled: boolean
  hasPassword?: boolean
  hasApiToken?: boolean
  lastSyncStatus?: string
  lastSyncMessage?: string
}
interface XUIRewriteRuleRow {
  nameContains: string
  protocol: string
  transport: string
  address: string
  port: string
  security: string
  sni: string
  host: string
  fingerprint: string
  alpn: string[]
  path: string
  flow: string
}
onMounted(async() => {  // 椤甸潰寮€濮嬫墽琛屽嚱鏁?   getnodes()
   GetGroups()
   getXUISources()
})
const dialogMode = ref<'add' | 'edit'>('add');

// --- 琛ㄦ牸閫夋嫨涓庢搷浣滅浉鍏虫暟鎹?---
const multipleSelection = ref<Node[]>([]); // Stores selected table items
const multipleTable = ref<any>(null)


const tableRefs = ref<{ [key: string]: any }>({}); // Stores references to each el-table
// --- 琛ㄦ牸閫夋嫨涓庢搷浣滅浉鍏虫暟鎹粨鏉?---
// const NodeNewLinkInput = ref("")
// const NodeNewNameInput = ref("")
const NodeGroupInput = ref("")
const tableData = ref<Node[]>([])
// 鍒嗙粍鍒楄〃涓存椂瀛樻斁鏁版嵁
const tableDataTemp = ref<Node[]>([])
// 鍒嗙粍鍒楄〃涓存椂瀛樻斁鏁版嵁
const activeName = ref('鍏ㄩ儴')
const Nodedialog = ref (false); // 寮圭獥鏄惁鍙
const Groupdialog = ref (false); // 寮圭獥鏄惁鍙
const NodeForm = ref<NodeInfo>({
    Title: '',
    Name: '',
    Link: '',
    GroupName: [],
  })
const allGroupNames = ref<string[]>([]); // 鎵€鏈夊垎缁勫悕绉?
const allNodes = ref<string[]>([]); // 鎵€鏈夎妭鐐?
const nodelistShow = ref(false); // 鑺傜偣鍒楄〃
const SelectionNodeGroups = ref<string[]>([]); // 閫変腑鐨勫垎缁?
const SelectionNode = ref(''); // 閫変腑鐨勮妭鐐?

// const SelectionNodes = ref([]); // 閫変腑鐨勮妭鐐?
const RadioGroup = ref("1"); // 鍒嗙粍鍗曢€夋
const SourceDialog = ref(false);
const showSourceAdvanced = ref(false);
const sourceAuthType = ref<'password' | 'apiToken'>('password');
const sourceTableData = ref<XUISource[]>([]);
const rewriteRuleRows = ref<XUIRewriteRuleRow[]>([]);
const sourceForm = ref<XUISource>({
  name: '',
  host: '',
  sshPort: 22,
  username: 'root',
  password: '',
  panelBaseUrl: '',
  apiToken: '',
  xuiDbPath: '/etc/x-ui/x-ui.db',
  subBaseUrl: 'https://127.0.0.1:2096',
  subPath: 'dingyue',
  groupName: '',
  namePrefix: '',
  rewriteRules: '',
  deleteMissing: false,
  enabled: true,
});
function newRewriteRuleRow(): XUIRewriteRuleRow {
  return {
    nameContains: '',
    protocol: '',
    transport: 'xhttp',
    address: '',
    port: '',
    security: 'tls',
    sni: '',
    host: '',
    fingerprint: 'chrome',
    alpn: ['h2', 'http/1.1'],
    path: '',
    flow: '',
  };
}
function addRewriteRule(row?: Partial<XUIRewriteRuleRow>) {
  rewriteRuleRows.value.push({
    ...newRewriteRuleRow(),
    ...row,
  });
}
function removeRewriteRule(index: number) {
  rewriteRuleRows.value.splice(index, 1);
}
function parseRewriteRules(raw: string) {
  rewriteRuleRows.value = [];
  const text = (raw || '').trim();
  if (!text) return;
  try {
    const parsed = JSON.parse(text);
    const rules = Array.isArray(parsed) ? parsed : [parsed];
    rewriteRuleRows.value = rules.map((rule: any) => ({
      ...newRewriteRuleRow(),
      nameContains: rule.nameContains || '',
      protocol: rule.protocol || '',
      transport: rule.transport || '',
      address: rule.address || '',
      port: rule.port || '',
      security: rule.security || '',
      sni: rule.sni || '',
      host: rule.host || '',
      fingerprint: rule.fp || rule.fingerprint || '',
      alpn: typeof rule.alpn === 'string' && rule.alpn
        ? rule.alpn.split(',').map((item: string) => item.trim()).filter(Boolean)
        : [],
      path: rule.path || '',
      flow: rule.flow || '',
    }));
  } catch (error) {
    console.warn('parse rewrite rules failed:', error);
  }
}
function serializeRewriteRules() {
  const rules = rewriteRuleRows.value.map((row) => {
    const rule: Record<string, string> = {};
    const setValue = (key: string, value: string) => {
      const next = (value || '').trim();
      if (next) rule[key] = next;
    };
    setValue('nameContains', row.nameContains);
    setValue('protocol', row.protocol);
    setValue('transport', row.transport);
    setValue('address', row.address);
    setValue('port', row.port);
    setValue('security', row.security);
    setValue('sni', row.sni);
    setValue('host', row.host);
    setValue('fp', row.fingerprint);
    setValue('alpn', row.alpn.join(','));
    setValue('path', row.path);
    setValue('flow', row.flow);
    return rule;
  }).filter((rule) => Object.keys(rule).length > 0);
  return rules.length ? JSON.stringify(rules) : '';
}
// 将所有输入的值清空
function ClearInput() {
  SelectionNode.value = ''; // 娓呯┖閫変腑鐨勮妭鐐?
  NodeForm.value = { // 娓呯┖鑺傜偣閾炬帴杈撳叆妗?
    Title: '',
    Name: '',
    Link: '',
    GroupName: [],
  }
  NodeGroupInput.value = ''; // 娓呯┖鍒涘缓鍒嗙粍杈撳叆妗?
  SelectionNodeGroups.value = []; // 娓呯┖閫変腑鐨勫垎缁?
  nodelistShow.value = false; // 闅愯棌鑺傜偣鍒楄〃
  Nodedialog.value = false; // 鍏抽棴鑺傜偣娣诲姞寮圭獥
  Groupdialog.value = false; // 鍏抽棴鍒嗙粍缁戝畾寮圭獥
  
}
async function getnodes() {
  const {data} = await getNodes();
  const nodes = Array.isArray(data) ? data : [];
  tableDataTemp.value = nodes;
  allNodes.value = nodes.map((item:any) => item.Name);
  applyActiveGroupFilter();
  
} 
function applyActiveGroupFilter() {
  if (activeName.value === '鍏ㄩ儴') {
    tableData.value = tableDataTemp.value;
    return;
  }
  tableData.value = tableDataTemp.value.filter(item => {
    return item.GroupNodes?.some(group => group.Name === activeName.value);
  });
}
async function syncXUINodes() {
  try {
    const { data } = await SyncXUINodes();
    ElMessage.success(`3x-ui sync: +${data.created} / ~${data.updated} / =${data.unchanged}`);
    if (data.skipped > 0) {
      console.warn('3x-ui sync skipped nodes:', data.skippedOn);
    }
    await getnodes();
    await GetGroups();
  } catch (error) {
    console.error('3x-ui sync failed:', error);
    ElMessage.error('3x-ui sync failed');
  }
}
function resetSourceForm() {
  sourceAuthType.value = 'password';
  rewriteRuleRows.value = [];
  sourceForm.value = {
    name: '',
    host: '',
    sshPort: 22,
    username: 'root',
    authType: 'password',
    password: '',
    panelBaseUrl: '',
    apiToken: '',
    xuiDbPath: '/etc/x-ui/x-ui.db',
    subBaseUrl: 'https://127.0.0.1:2096',
    subPath: 'dingyue',
    groupName: '',
    namePrefix: '',
    rewriteRules: '',
    deleteMissing: false,
    enabled: true,
  };
}
async function getXUISources() {
  const { data } = await GetXUISources();
  sourceTableData.value = Array.isArray(data) ? data : [];
}
async function openSourceDialog() {
  SourceDialog.value = true;
  showSourceAdvanced.value = false;
  sourceAuthType.value = 'password';
  rewriteRuleRows.value = [];
  await getXUISources();
}
function editXUISource(row: any) {
  sourceAuthType.value = row.authType === 'apiToken' ? 'apiToken' : 'password';
  showSourceAdvanced.value = !!row.rewriteRules;
  parseRewriteRules(row.rewriteRules || '');
  sourceForm.value = {
    ...row,
    authType: sourceAuthType.value,
    password: '',
    apiToken: '',
  };
}
async function saveXUISource() {
  try {
    const payload = {
      ...sourceForm.value,
      authType: sourceAuthType.value,
      password: sourceAuthType.value === 'password' ? sourceForm.value.password : '',
      apiToken: sourceAuthType.value === 'apiToken' ? sourceForm.value.apiToken : '',
      subBaseUrl: showSourceAdvanced.value ? sourceForm.value.subBaseUrl : '',
      subPath: showSourceAdvanced.value ? sourceForm.value.subPath : '',
      xuiDbPath: showSourceAdvanced.value ? sourceForm.value.xuiDbPath : '',
      groupName: showSourceAdvanced.value ? sourceForm.value.groupName : '',
      namePrefix: showSourceAdvanced.value ? sourceForm.value.namePrefix : '',
      rewriteRules: serializeRewriteRules(),
    };
    await SaveXUISource(payload);
    ElMessage.success('VPS 源已保存');
    resetSourceForm();
    await getXUISources();
  } catch (error) {
    console.error('save x-ui source failed:', error);
    ElMessage.error('VPS 源保存失败');
  }
}
async function deleteXUISource(row: any) {
  if (!row.id) return;
  try {
    await ElMessageBox.confirm(`删除 VPS 源 ${row.name} ?`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    });
    await DelXUISource({ id: row.id });
    ElMessage.success('VPS 源已删除');
    await getXUISources();
  } catch (error) {
    if (error !== 'cancel') {
      console.error('delete x-ui source failed:', error);
      ElMessage.error('VPS 源删除失败');
    }
  }
}
async function syncXUISource(row: any) {
  if (!row.id) return;
  try {
    const { data } = await SyncXUISource(row.id);
    ElMessage.success(`${row.name}: +${data.created} / ~${data.updated} / =${data.unchanged}`);
    await getnodes();
    await GetGroups();
    await getXUISources();
  } catch (error) {
    console.error('sync x-ui source failed:', error);
    ElMessage.error(`${row.name} 同步失败`);
    await getXUISources();
  }
}
async function syncAllXUISources() {
  try {
    await SyncAllXUISources();
    ElMessage.success('已同步全部启用的 VPS 源');
    await getnodes();
    await GetGroups();
    await getXUISources();
  } catch (error) {
    console.error('sync all x-ui sources failed:', error);
    ElMessage.error('同步全部 VPS 源失败');
  }
}
async function GetGroups() {
  const {data} = await GetGroup();
  allGroupNames.value = Array.isArray(data) ? data : [];
  if (activeName.value !== '全部' && !allGroupNames.value.includes(activeName.value)) {
    activeName.value = '全部';
  }
  RadioGroup.value = allGroupNames.value.length > 0 ? "1" : "2";
  applyActiveGroupFilter();
  
}


const handleAddNode = () => {
  dialogMode.value = 'add';
  Nodedialog.value = true;
  NodeForm.value = {
    Title: '添加节点',
    Name: '',
    Link: '',
    GroupName: [],
  };
  SelectionNodeGroups.value = [];
  NodeGroupInput.value = '';
};

const handleEditNode = (row: any) => {  
  // NodeNewNameInput.value = row.Name; // 缂栬緫鏃朵娇鐢ㄥ師鍚嶇О
  // NodeNewLinkInput.value = row.Link; // 缂栬緫鏃朵娇鐢ㄥ師閾炬帴
  dialogMode.value = 'edit';
  Nodedialog.value = true;
  NodeForm.value = {
    ID: row.ID,
    Title: '编辑节点',
    Name: row.Name,
    Link: row.Link,
    GroupName: (row.GroupNodes || []).map((g: any) => g.Name),
  };
  SelectionNodeGroups.value = NodeForm.value.GroupName || [];
  SelectionNode.value = row.Name;
};
const SubmitNodeForm = async (row:any) => {
  const isAdd = dialogMode.value === 'add';
  let links = NodeForm.value.Link.trim().split(/[\n,]/).map(item => item.trim()).filter(item => item);
  if (isAdd && links.length === 0) {
    ElMessage.warning('节点链接不能为空');
    return;
  }

  try {
    if (isAdd) {
      for (const link of links) {
        await AddNodes({
          link,
          group: RadioGroup.value === '1' ? SelectionNodeGroups.value.join(',') : NodeGroupInput.value,
        });
      }
      ElMessage.success('节点添加成功');
    } else {
      await UpdateNode({
        id:NodeForm.value.ID,
        name: NodeForm.value.Name, // 鏂板悕绉?
        link: NodeForm.value.Link, // 鏂伴摼鎺?
        group: RadioGroup.value === '1' ? SelectionNodeGroups.value.join(',') : NodeGroupInput.value,
      });
      ElMessage.success('节点更新成功');
    }


  } catch (err) {
    ElMessage.error(`${isAdd ? '添加' : '更新'}失败`);
  }
  getnodes();
  GetGroups();
  ClearInput();
};

// const AddNode = async() => {
//   // 澶氳妭鐐归摼鎺ヨ緭鍏ュ鐞?
//   let NodeLinkInputs = NodeNewLinkInput.value.trim().split(/[\n,]/); // 浣跨敤鎹㈣绗︽垨閫楀彿鍒嗛殧杈撳叆鐨勮妭鐐归摼鎺?
//   NodeLinkInputs = NodeLinkInputs.map((item) => item.trim()).filter((item) => item !== ''); // 鍘婚櫎绌虹櫧鍜岄噸澶嶇殑閾炬帴
//   if (NodeNewLinkInput.value.trim() === '') {
//     ElMessage.warning('鑺傜偣閾炬帴涓嶈兘涓虹┖');
//     return;
//   }

//   try {
//     // 澶氳妭鐐瑰悓姝ュ惊鐜坊鍔犺妭鐐?
//     for(const link of NodeLinkInputs) {
//       if (link) {
//           const newNode = {
//           link: link.trim(), // 鑺傜偣閾炬帴
//           group: SelectionNodeGroups.value.join(','), // 閫変腑鐨勫垎缁?
//           };
//           await AddNodes(newNode).then(() => {
//           ElMessage.success('鑺傜偣娣诲姞鎴愬姛');
//           Nodedialog.value = false; // 鍏抽棴寮圭獥
//           });
//       }
//     }
//     // getnodes(); // 鍒锋柊鑺傜偣鍒楄〃
//     // GetGroups(); // 鍒锋柊鍒嗙粍鍒楄〃
//   } catch (error) {
//     console.error('娣诲姞鑺傜偣澶辫触:', error);
//     // ElMessage.error('娣诲姞鑺傜偣澶辫触锛岃绋嶅悗鍐嶈瘯');
//   }
//   getnodes(); // 鍒锋柊鑺傜偣鍒楄〃
//   GetGroups(); // 鍒锋柊鍒嗙粍鍒楄〃
//   ClearInput(); // 娓呯┖鎵€鏈夎緭鍏?
// }
const AddGroup = async() => {
  console.log(SelectionNode.value);

  try {
    // 妫€鏌ユ槸鍚﹂€夋嫨浜嗗凡鏈夊垎缁勬垨杈撳叆浜嗘柊鍒嗙粍鍚?
    console.log(RadioGroup.value, SelectionNodeGroups.value, NodeGroupInput.value);
    
    if (RadioGroup.value === "1" && SelectionNodeGroups.value.length === 0) {
      ElMessage.warning('你还没有选择分组');
      return;
    }
    if (RadioGroup.value === "2"&&NodeGroupInput.value.trim() === '') {
      ElMessage.warning('创建的分组名不能为空');
      return;
  }
      if (SelectionNode.value.length > 0) { // 濡傛灉娌℃湁閫夋嫨鑺傜偣
      const newNode = {
      name: SelectionNode.value, // 鑺傜偣閾炬帴
      group: RadioGroup.value == '1' ?SelectionNodeGroups.value.join(','):NodeGroupInput.value, // 鏉′欢閫夋嫨宸叉湁鑺傜偣|鍒涘缓鍒嗙粍
      };
      await SetGroup(newNode).then(() => {
      ElMessage.success('分组绑定成功');
            });
    }
  } catch (error) {
    console.error('娣诲姞鍒嗙粍澶辫触:', error);
    // ElMessage.error('娣诲姞鍒嗙粍澶辫触');
  }
  getnodes(); // 鍒锋柊鑺傜偣鍒楄〃
  GetGroups(); // 鍒锋柊鍒嗙粍鍒楄〃
  ClearInput(); // 娓呯┖鎵€鏈夎緭鍏?
}
// 琛ㄦ牸鏃堕棿鏍煎紡鍖?
const Timeformatter  = (row:any)=>{
  row.CreatedAt = new Date(row.CreatedAt).toLocaleString(); // 杞崲涓烘湰鍦版椂闂村瓧绗︿覆
  return row.CreatedAt;
  
}
// 閫夋嫨宸叉湁鑺傜偣鏄剧ず鎵€灞炲垎缁?
const  handleShownodeGroupList =()=>{
  // 鏄剧ず杩欎釜鑺傜偣鍏宠仈鐨勫垎缁?
  const nodeData = allNodes.value.find(node => node === SelectionNode.value);
  SelectionNodeGroups.value = []
  tableData.value.forEach((item, ) => {
    if (item.Name === nodeData && (item.GroupNodes?.length ?? 0) > 0) {
      // console.log(`鑺傜偣 ${nodeData} 鐨勫垎缁?`, item.GroupNodes);
      item.GroupNodes?.forEach((item) => {
        SelectionNodeGroups.value.push(item.Name); // 灏嗗垎缁勫悕绉版坊鍔犲埌 SelectionNodeGroups 涓?
      });
    } 
});
}
// 琛ㄦ牸鎵€灞炲垎缁勬牸寮忓寲
const Groupformatter = (row:any,cellValue:any) =>{
  const data = row.GroupNodes || [];
  if (!Array.isArray(data) || data.length === 0) {
    return '未分组';
  }
 return data.map((group: any) => group.Name).join(', ');
}
// --- 澶嶅埗閾炬帴 (淇濇寔涓嶅彉) ---
const copyUrl = (url: string) => {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(url)
      .then(() => {
        ElMessage.success('链接已复制到剪贴板');
      })
      .catch(err => {
        console.error('复制失败:', err);
        ElMessage.error('复制失败，请手动复制');
      });
  } else {
    const textarea = document.createElement('textarea');
    textarea.value = url;
    document.body.appendChild(textarea);
    textarea.select();
    try {
      document.execCommand('copy');
      ElMessage.success('链接已复制到剪贴板');
    } catch (err) {
      ElMessage.warning('复制失败');
    } finally {
      document.body.removeChild(textarea);
    }
  }
};
// 澶嶅埗琛ㄦ牸鑺傜偣淇℃伅
const copyInfo = (row: any) => {
  copyUrl(row.Link);
};
const handleDel = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `你是否要删除 ${row.Name} ?`,
      '提示',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    );
    await DelNode({ id: row.ID });
    ElMessage.success('删除成功');
  } catch (error) {
    if (error !== 'cancel') {
      console.error("删除失败:", error);
      ElMessage.error('删除失败');
    }
  }
  // 鍒锋柊鑺傜偣鍒楄〃
  await GetGroups(); // 鍒锋柊鍒嗙粍鍒楄〃
  await getnodes(); // 鍒锋柊鑺傜偣鍒楄〃
  ClearInput(); // 娓呯┖鎵€鏈夎緭鍏?
};
const selectDel = async () => {
  
  if (multipleSelection.value.length === 0) {
    ElMessage.warning('请选择要删除的节点');
    return;
  }
  try {
    await ElMessageBox.confirm(
      `你是否要删除选中的 ${multipleSelection.value.length} 条节点?`,
      '提示',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    );

    const IDs: number[] = []

    for (const item of multipleSelection.value) {
      await DelNode({ id: item.ID });
       IDs.push(item.ID); // 鏀堕泦鎵€鏈夊凡鍒犻櫎鐨勮妭鐐笽D
    }
    ElMessage.success('批量删除成功');
    // 浠?tableData 涓垹闄ゅ凡鍒犻櫎鐨勮妭鐐?
    tableData.value = tableData.value.filter(item => !IDs.includes(item.ID));
; 

  } catch (error) {
    if (error !== 'cancel') {
      console.error("批量删除失败:", error);
      ElMessage.error('批量删除失败');
    }
  }
    // 鍒锋柊鑺傜偣鍒楄〃
  await GetGroups(); // 鍒锋柊鍒嗙粍鍒楄〃
  await getnodes();
};
// 鍏ㄩ€?
const selectAll = () => {
  nextTick(() => {
const table = multipleTable.value
  if (table) {
      // 鍚﹀垯鍏ㄩ€?
      tableData.value.forEach(row => {
        table.toggleRowSelection(row, true)
      })
  }
  });
};
// 鍙栨秷鍏ㄩ€?
const selectClear = () => {
  nextTick(() => {
    const table = multipleTable.value;
    if (table) {
      table.clearSelection();
    }
  });
};
// --- 琛ㄦ牸閫夋嫨鎿嶄綔 (淇濇寔涓嶅彉) ---
const setTableRef = (el: any, name: string) => {
  if (el) {
    tableRefs.value[name] = el;
  } else {
    delete tableRefs.value[name];
  }
};
//鎵归噺澶嶅埗
const selectCopy = async () => {
  if (multipleSelection.value.length === 0) {
    ElMessage.warning('请选择要复制的节点');
    return;
  }
  try {
    copyUrl(multipleSelection.value.map(item => item.Link).join('\n'));
  } catch (error) {
    if (error !== 'cancel') {
      console.error("批量复制失败:", error);
      ElMessage.error('批量复制失败');
    }
  }
};
const handleSelectionChange = (val: Node[]) => {
  multipleSelection.value = val;
};

watch(activeName, () => {
  applyActiveGroupFilter();
});


</script>

<template>
  <div>
 <el-dialog v-model="Nodedialog" :title="NodeForm.Title" width="80%">
  <el-input
    v-model="NodeForm.Link"
    placeholder="请输入节点链接，支持多行使用回车或逗号分开"
    type="textarea"
    style="margin-bottom: 10px"
    :autosize="{ minRows: 2, maxRows: 10 }"
    v-if="dialogMode === 'add'"
  />

<el-input
  v-model="NodeForm.Name"
  placeholder="节点名称"
  style="margin-bottom: 10px"
  v-if="dialogMode === 'edit'"
/>
  <el-input
    v-model="NodeForm.Link"
    placeholder="请输入节点链接，支持多行使用回车或逗号分开"
    type="textarea"
    style="margin-bottom: 10px"
    :autosize="{ minRows: 2, maxRows: 10 }"
    v-if="dialogMode === 'edit'"
  />

  <!-- 鍒嗙粍閮ㄥ垎 -->
  <el-radio v-model="RadioGroup" label="1" v-if="allGroupNames.length > 0">选择已有分组</el-radio>
  <el-radio v-model="RadioGroup" label="2">创建新分组</el-radio>

  <div v-if="RadioGroup === '1' && allGroupNames.length > 0">
    <el-select v-model="SelectionNodeGroups" multiple placeholder="选择已有分组" class="default">
      <el-option v-for="item in allGroupNames" :key="item" :label="item" :value="item" />
    </el-select>
  </div>

  <el-input v-if="RadioGroup === '2'" v-model="NodeGroupInput" placeholder="输入要创建的分组名" class="default" />

  <el-button type="primary" @click="SubmitNodeForm">{{ dialogMode === 'add' ? '添加' : '更新' }}</el-button>
  <el-button @click="Nodedialog = false">取消</el-button>
</el-dialog>

<el-dialog v-model="SourceDialog" title="VPS 源管理" width="92%">
  <el-form :model="sourceForm" label-width="130px">
    <el-row :gutter="12">
      <el-col :span="8">
        <el-form-item label="名称">
          <el-input v-model="sourceForm.name" placeholder="请输入 VPS 名称" />
        </el-form-item>
      </el-col>
      <el-col :span="8" v-if="sourceAuthType === 'password'">
        <el-form-item label="主机">
          <el-input v-model="sourceForm.host" placeholder="1.2.3.4" />
        </el-form-item>
      </el-col>
      <el-col :span="8" v-if="sourceAuthType === 'apiToken'">
        <el-form-item label="面板地址">
          <el-input v-model="sourceForm.panelBaseUrl" placeholder="https://panel.example.com:54321" />
        </el-form-item>
      </el-col>
      <el-col :span="4" v-if="sourceAuthType === 'password'">
        <el-form-item label="SSH 端口">
          <el-input v-model.number="sourceForm.sshPort" />
        </el-form-item>
      </el-col>
      <el-col :span="4">
        <el-form-item label="启用">
          <el-switch v-model="sourceForm.enabled" />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="12">
      <el-col :span="6" v-if="sourceAuthType === 'password'">
        <el-form-item label="用户名">
          <el-input v-model="sourceForm.username" />
        </el-form-item>
      </el-col>
      <el-col :span="6">
        <el-form-item label="认证方式">
          <el-radio-group v-model="sourceAuthType">
            <el-radio-button label="password">账号密码</el-radio-button>
            <el-radio-button label="apiToken">API Token</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-col>
      <el-col :span="12" v-if="sourceAuthType === 'password'">
        <el-form-item label="密码">
          <el-input v-model="sourceForm.password" type="password" show-password placeholder="编辑时留空则保留原密码" />
        </el-form-item>
      </el-col>
      <el-col :span="12" v-if="sourceAuthType === 'apiToken'">
        <el-form-item label="API Token">
          <el-input v-model="sourceForm.apiToken" type="password" show-password placeholder="编辑时留空则保留原 Token" />
        </el-form-item>
      </el-col>
    </el-row>
    <el-button text type="primary" @click="showSourceAdvanced = !showSourceAdvanced">
      {{ showSourceAdvanced ? '收起高级选项' : '高级选项' }}
    </el-button>
    <template v-if="showSourceAdvanced">
      <el-row :gutter="12">
        <el-col :span="8" v-if="sourceAuthType === 'password'">
          <el-form-item label="x-ui DB">
            <el-input v-model="sourceForm.xuiDbPath" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="订阅 Base">
            <el-input v-model="sourceForm.subBaseUrl" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="订阅路径">
            <el-input v-model="sourceForm.subPath" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="8">
          <el-form-item label="分组">
            <el-input v-model="sourceForm.groupName" placeholder="默认同名称" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="名称前缀">
            <el-input v-model="sourceForm.namePrefix" placeholder="默认使用 [VPS名称] " />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="删除缺失节点">
            <el-switch v-model="sourceForm.deleteMissing" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item label="节点改写规则">
        <div class="rewrite-rule-panel">
          <el-button type="primary" size="small" @click="addRewriteRule()">+ 添加规则</el-button>
          <div v-if="rewriteRuleRows.length === 0" class="rewrite-rule-empty">未添加改写规则，同步时保持原始节点参数。</div>
          <div v-for="(rule, index) in rewriteRuleRows" :key="index" class="rewrite-rule-item">
            <div class="rewrite-rule-line">
              <el-select v-model="rule.transport" clearable placeholder="传输" class="rewrite-rule-control small">
                <el-option label="XHTTP" value="xhttp" />
                <el-option label="TCP" value="tcp" />
                <el-option label="WS" value="ws" />
                <el-option label="gRPC" value="grpc" />
              </el-select>
              <el-select v-model="rule.protocol" clearable placeholder="协议" class="rewrite-rule-control small">
                <el-option label="VLESS" value="vless" />
                <el-option label="VMess" value="vmess" />
                <el-option label="Trojan" value="trojan" />
              </el-select>
              <el-input v-model="rule.nameContains" placeholder="名称包含" class="rewrite-rule-control" />
              <el-input v-model="rule.address" placeholder="地址，不填不改" class="rewrite-rule-control" />
              <el-input v-model="rule.port" placeholder="端口" class="rewrite-rule-control port" />
              <el-button circle @click="removeRewriteRule(index)">-</el-button>
            </div>
            <div class="rewrite-rule-line">
              <el-select v-model="rule.security" clearable placeholder="安全" class="rewrite-rule-control small">
                <el-option label="TLS" value="tls" />
                <el-option label="Reality" value="reality" />
                <el-option label="None" value="none" />
              </el-select>
              <el-input v-model="rule.sni" placeholder="SNI" class="rewrite-rule-control" />
              <el-input v-model="rule.host" placeholder="Host" class="rewrite-rule-control" />
              <el-select v-model="rule.fingerprint" clearable placeholder="Fingerprint" class="rewrite-rule-control small">
                <el-option label="chrome" value="chrome" />
                <el-option label="firefox" value="firefox" />
                <el-option label="safari" value="safari" />
                <el-option label="random" value="random" />
              </el-select>
              <el-select v-model="rule.alpn" multiple collapse-tags placeholder="ALPN" class="rewrite-rule-control alpn">
                <el-option label="h2" value="h2" />
                <el-option label="http/1.1" value="http/1.1" />
              </el-select>
            </div>
            <div class="rewrite-rule-line">
              <el-input v-model="rule.path" placeholder="Path，不填不改" class="rewrite-rule-control" />
              <el-input v-model="rule.flow" placeholder="Flow，不填不改" class="rewrite-rule-control" />
            </div>
          </div>
        </div>
      </el-form-item>
    </template>
    <el-button type="primary" @click="saveXUISource">保存 VPS 源</el-button>
    <el-button @click="resetSourceForm">清空表单</el-button>
    <el-button type="success" @click="syncAllXUISources">同步全部启用源</el-button>
  </el-form>
  <el-table :data="sourceTableData" stripe style="width: 100%; margin-top: 16px">
    <el-table-column prop="name" label="名称" />
    <el-table-column label="地址">
      <template #default="{row}">
        {{ row.authType === 'apiToken' ? row.panelBaseUrl : row.host }}
      </template>
    </el-table-column>
    <el-table-column prop="groupName" label="分组" />
    <el-table-column label="接入">
      <template #default="{row}">
        <el-tag v-if="row.authType === 'apiToken'" type="warning">API Token</el-tag>
        <el-tag v-else type="success">账号密码</el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="lastSyncStatus" label="状态" />
    <el-table-column prop="lastSyncMessage" label="消息" show-overflow-tooltip />
    <el-table-column label="操作" width="220">
      <template #default="{row}">
        <el-button link type="primary" @click="editXUISource(row)">编辑</el-button>
        <el-button link type="success" @click="syncXUISource(row)">同步</el-button>
        <el-button link type="danger" @click="deleteXUISource(row)">删除</el-button>
      </template>
    </el-table-column>
  </el-table>
</el-dialog>


  <!-- 鏄剧ず琛ㄦ牸鏁版嵁 -->
  <el-card>
    <el-tabs v-model="activeName" >
      <el-tab-pane :label="`全部(${allNodes.length})`" name="全部" />
      <el-tab-pane :label="item" :name="item" v-for="item in allGroupNames" :key="item" />
    </el-tabs>
      <el-button type="primary" @click="handleAddNode">添加节点</el-button>
      <el-button type="success" @click="syncXUINodes">同步 3x-ui 节点</el-button>
      <el-button type="warning" @click="openSourceDialog">VPS 源管理</el-button>
      <div style="margin-bottom: 10px"></div>
      <el-table
      ref="multipleTable"
    :data="tableData"
    tooltip-effect="dark"
    stripe
    style="width: 100%"
    row-key="ID" 
    :tree-props="{children: 'Nodes'}"
    @selection-change="handleSelectionChange"
    >
        <el-table-column
      type="selection"
      width="55">
    </el-table-column>

    <el-table-column
      type="index"
      >
    </el-table-column>
    <el-table-column
      prop="Name"
      label="节点名称"
      sortable
      >
    <template #default="{row}">
      <el-tag effect="plain" >{{row.Name}}</el-tag>
        </template>
    </el-table-column>
    <el-table-column
      prop="Link"
      label="链接"
      :show-overflow-tooltip="true"
      >
          <template #default="{row}">
      <el-tag effect="plain" type="success" >{{row.Link}}</el-tag>
        </template>
    </el-table-column>
    
        <el-table-column
      prop="CreatedAt"
      label="创建时间"
      :formatter="Timeformatter"
      sortable
      show-overflow-tooltip>
    </el-table-column>
            <el-table-column
      label="所属分组"
      :formatter="Groupformatter"
      show-overflow-tooltip>
    </el-table-column>
                <el-table-column  label="鎿嶄綔" width="120">
              <template #default="scope">
                <el-button link type="primary" size="small" @click="handleEditNode(scope.row)">编辑</el-button>
                <el-button link type="primary" size="small" @click="copyInfo(scope.row)">复制</el-button>
                <el-button link type="primary" size="small" @click="handleDel(scope.row)">删除</el-button>
              </template>
            </el-table-column>
  </el-table>
   <div style="margin-top: 20px" />
   <el-button type="info" @click="selectAll">全选</el-button>
   <el-button type="warning" @click="selectClear">取消选中</el-button>
      <el-button type="primary" @click="selectCopy">复制选中</el-button>
      <el-button type="danger" @click="selectDel">删除选中</el-button>
      <div style="margin-top: 20px" />
  </el-card>
  <!-- 鏄剧ず琛ㄦ牸鏁版嵁缁撴潫 -->
  </div>
</template>
<style>
 /* 鍒涘缓榛樿鏍峰紡 */
 .default {
  font-size: 14px;
  color: #333;
  line-height: 1.6;
  margin-bottom: 10px;
 }
 .rewrite-rule-panel {
  width: 100%;
 }
 .rewrite-rule-empty {
  color: #909399;
  font-size: 13px;
  margin-top: 8px;
 }
 .rewrite-rule-item {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 10px;
  margin-top: 10px;
 }
 .rewrite-rule-line {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
  flex-wrap: wrap;
 }
 .rewrite-rule-line:last-child {
  margin-bottom: 0;
 }
 .rewrite-rule-control {
  width: 180px;
 }
 .rewrite-rule-control.small {
  width: 130px;
 }
 .rewrite-rule-control.port {
  width: 100px;
 }
 .rewrite-rule-control.alpn {
  width: 190px;
 }
</style>

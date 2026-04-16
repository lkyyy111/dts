// imageStorage 统一处理图片在应用沙盒和系统相册之间的复制、下载与权限细节。
import * as FileSystem from "expo-file-system/legacy";
import * as MediaLibrary from "expo-media-library";

// 当源地址里看不出后缀时，默认按 jpg 处理。
const DEFAULT_IMAGE_EXT = ".jpg";

// 缓存相册权限，避免连续保存时反复弹权限检查带来卡顿。
let albumPermissionGranted = false;

// isRemoteImageUri 用来判断传入图片是否仍然是网络地址。
export function isRemoteImageUri(uri: string) {
  return /^https?:\/\//i.test(uri);
}

// 只有 file:// 开头的本地文件路径才能直接写进系统相册。
function isDirectAlbumUri(uri: string) {
  return /^file:\/\//i.test(uri);
}

// 从原始地址里尽量推断文件后缀，便于保存时保留正确格式。
function getFileExt(uri: string) {
  const cleanUri = uri.split("?")[0].split("#")[0];
  const match = cleanUri.match(/\.([a-zA-Z0-9]{2,8})$/);
  return match ? `.${match[1].toLowerCase()}` : DEFAULT_IMAGE_EXT;
}

// ensureDir 会在写文件前先保证目标目录已经存在。
async function ensureDir(targetDir: string) {
  const dirInfo = await FileSystem.getInfoAsync(targetDir);
  if (!dirInfo.exists) {
    await FileSystem.makeDirectoryAsync(targetDir, { intermediates: true });
  }
}

// getAppStoragePath 把相对路径拼成应用 documentDirectory 下的绝对路径。
function getAppStoragePath(relativePath: string) {
  if (!FileSystem.documentDirectory) {
    return "";
  }
  const baseDir = FileSystem.documentDirectory.endsWith("/")
    ? FileSystem.documentDirectory
    : `${FileSystem.documentDirectory}/`;
  return `${baseDir}${relativePath.replace(/^\/+/, "")}`;
}

// ensureParentDirForFile 会在写文件前补齐父目录。
async function ensureParentDirForFile(targetPath: string) {
  const normalized = targetPath.replace(/\\/g, "/");
  const lastSlashIndex = normalized.lastIndexOf("/");
  if (lastSlashIndex < 0) {
    return;
  }
  const parentDir = normalized.slice(0, lastSlashIndex);
  if (!parentDir) {
    return;
  }
  await ensureDir(parentDir);
}

// saveImageToAppRelativePath 会把图片写入应用沙盒中的固定相对路径。
export async function saveImageToAppRelativePath(
  uri: string,
  relativePath: string,
) {
  const targetPath = getAppStoragePath(relativePath);
  if (!targetPath) {
    return uri;
  }

  const normalizedSourceUri = uri.replace(/\\/g, "/");
  const normalizedTargetPath = targetPath.replace(/\\/g, "/");
  if (normalizedSourceUri === normalizedTargetPath) {
    return targetPath;
  }

  try {
    await ensureParentDirForFile(targetPath);
    const existed = await FileSystem.getInfoAsync(targetPath);
    if (existed.exists) {
      await FileSystem.deleteAsync(targetPath, { idempotent: true });
    }

    if (isRemoteImageUri(uri) && FileSystem.downloadAsync) {
      const downloaded = await FileSystem.downloadAsync(uri, targetPath);
      return downloaded.uri || targetPath;
    }

    await FileSystem.copyAsync({ from: uri, to: targetPath });
    return targetPath;
  } catch {
    return uri;
  }
}

// saveImageToLocalDir 会为图片生成一个随机文件名并存入指定目录。
export async function saveImageToLocalDir(uri: string, folderName: string) {
  if (!FileSystem.documentDirectory) {
    return uri;
  }

  const fileName = `${Date.now()}-${Math.random()
    .toString(36)
    .slice(2)}${getFileExt(uri)}`;
  return saveImageToAppRelativePath(uri, `${folderName}/${fileName}`);
}

// ensureAlbumPermission 统一处理系统相册权限的读取和申请。
async function ensureAlbumPermission() {
  if (albumPermissionGranted) {
    return;
  }

  const currentPermission = await MediaLibrary.getPermissionsAsync();
  if (currentPermission.granted) {
    albumPermissionGranted = true;
    return;
  }

  const nextPermission = currentPermission.canAskAgain
    ? await MediaLibrary.requestPermissionsAsync()
    : currentPermission;

  if (!nextPermission.granted) {
    throw new Error("请先允许应用访问相册，然后再执行保存。");
  }

  albumPermissionGranted = true;
}

// ensureNamedAlbum 会尽量把资源放进指定名称的系统相册里。
// 如果相册还不存在，就用当前资源创建；如果已存在，就把资源追加进去。
async function ensureNamedAlbum(asset: MediaLibrary.Asset, albumName: string) {
  const album = await MediaLibrary.getAlbumAsync(albumName);

  if (!album) {
    await MediaLibrary.createAlbumAsync(albumName, asset, false);
    return;
  }

  await MediaLibrary.addAssetsToAlbumAsync([asset], album, false);
}

// saveImageToAlbum 会尽量少复制一次文件，再把图片写入系统相册。
// 这里不只保存到系统图库，还会尽量归档到调用方指定的相册名称下。
export async function saveImageToAlbum(uri: string, folderName: string) {
  await ensureAlbumPermission();

  const localImageUri = isDirectAlbumUri(uri)
    ? uri
    : await saveImageToLocalDir(uri, folderName);
  if (!isDirectAlbumUri(localImageUri)) {
    throw new Error("图片未能成功转成本地文件，暂时无法写入系统相册。");
  }

  const asset = await MediaLibrary.createAssetAsync(localImageUri);
  await ensureNamedAlbum(asset, folderName);
  return localImageUri;
}

#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');
const { promisify } = require('util');
const stream = require('stream');
const pipeline = promisify(stream.pipeline);

const REPO_OWNER = 'mpwhite';
const REPO_NAME = 'river-go';
const BINARY_NAME = 'river';

async function getPlatformBinary() {
  const platform = process.platform;
  const arch = process.arch;
  
  const platformMap = {
    'darwin-x64': 'darwin-amd64',
    'darwin-arm64': 'darwin-arm64',
    'linux-x64': 'linux-amd64',
    'linux-arm64': 'linux-arm64',
    'win32-x64': 'windows-amd64',
  };
  
  const platformKey = `${platform}-${arch}`;
  const goPlatform = platformMap[platformKey];
  
  if (!goPlatform) {
    throw new Error(`Unsupported platform: ${platformKey}`);
  }
  
  return {
    platform: goPlatform,
    extension: platform === 'win32' ? '.exe' : ''
  };
}

async function downloadBinary(url, destPath) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { 'User-Agent': 'npm-installer' } }, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        downloadBinary(response.headers.location, destPath)
          .then(resolve)
          .catch(reject);
        return;
      }
      
      if (response.statusCode !== 200) {
        reject(new Error(`Failed to download: ${response.statusCode}`));
        return;
      }
      
      const file = fs.createWriteStream(destPath);
      response.pipe(file);
      
      file.on('finish', () => {
        file.close();
        resolve();
      });
      
      file.on('error', (err) => {
        fs.unlink(destPath, () => {});
        reject(err);
      });
    }).on('error', reject);
  });
}

async function getLatestRelease() {
  return new Promise((resolve, reject) => {
    const options = {
      hostname: 'api.github.com',
      path: `/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest`,
      headers: {
        'User-Agent': 'npm-installer',
        'Accept': 'application/vnd.github.v3+json'
      }
    };
    
    https.get(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const release = JSON.parse(data);
          resolve(release);
        } catch (err) {
          reject(err);
        }
      });
    }).on('error', reject);
  });
}

async function installFromSource() {
  console.log('No pre-built binary available. Building from source...');
  console.log('This requires Go to be installed on your system.');
  
  try {
    // Check if Go is installed
    execSync('go version', { stdio: 'ignore' });
    
    // Build the binary
    const binDir = path.join(__dirname, '..', 'bin');
    const binPath = path.join(binDir, BINARY_NAME);
    
    console.log('Building River...');
    execSync('go build -o ' + binPath, {
      cwd: path.join(__dirname, '..'),
      stdio: 'inherit'
    });
    
    // Make it executable
    if (process.platform !== 'win32') {
      fs.chmodSync(binPath, 0o755);
    }
    
    console.log('River built successfully!');
  } catch (err) {
    console.error('Failed to build from source. Please ensure Go is installed.');
    console.error('Visit https://golang.org/dl/ to install Go.');
    process.exit(1);
  }
}

async function install() {
  try {
    const binDir = path.join(__dirname, '..', 'bin');
    
    // Try to download pre-built binary first
    try {
      const { platform, extension } = await getPlatformBinary();
      const release = await getLatestRelease();
      
      // Find the asset for this platform
      const assetName = `river-${platform}${extension}`;
      const asset = release.assets.find(a => a.name === assetName);
      
      if (asset) {
        console.log(`Downloading River ${release.tag_name} for ${platform}...`);
        
        const binPath = path.join(binDir, BINARY_NAME + extension);
        await downloadBinary(asset.browser_download_url, binPath);
        
        // Make it executable
        if (process.platform !== 'win32') {
          fs.chmodSync(binPath, 0o755);
        }
        
        console.log('River installed successfully!');
        return;
      }
    } catch (err) {
      console.log('Could not download pre-built binary:', err.message);
    }
    
    // Fall back to building from source
    await installFromSource();
    
  } catch (err) {
    console.error('Installation failed:', err.message);
    process.exit(1);
  }
}

// Run installation
install();
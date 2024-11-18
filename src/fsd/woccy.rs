use anyhow::{ensure, Result};
use std::{
    collections::HashMap,
    fs::{self, DirEntry},
    path::{Path, PathBuf},
};

#[derive(Debug)]
struct WalkDirOptions {
    follow_links: bool,
    follow_root_links: bool,
    max_open: usize,
    min_depth: usize,
    max_depth: usize,
}

impl Default for WalkDirOptions {
    fn default() -> Self {
        Self {
            follow_links: false,
            follow_root_links: true,
            max_open: 10,
            min_depth: 0,
            max_depth: 10,
        }
    }
}

#[derive(Debug)]
pub struct WalkDir {
    opts: WalkDirOptions,
    root: PathBuf,
}

impl WalkDir {
    pub fn new<P: AsRef<Path>>(root: P) -> Self {
        Self {
            opts: WalkDirOptions::default(),
            root: root.as_ref().to_path_buf(),
        }
    }

    /// Follow symbolic links if these are the root of the traversal.
    /// By default, this is enabled.
    ///
    /// When `yes` is `true`, symbolic links on root paths are followed
    /// which is effective if the symbolic link points to a directory.
    /// If a symbolic link is broken or is involved in a loop, an error is yielded
    /// as the first entry of the traversal.
    ///
    /// When enabled, the yielded [`DirEntry`] values represent the target of
    /// the link while the path corresponds to the link. See the [`DirEntry`]
    /// type for more details, and all future entries will be contained within
    /// the resolved directory behind the symbolic link of the root path.
    ///
    /// [`DirEntry`]: struct.DirEntry.html
    pub fn follow_root_links(mut self, yes: bool) -> Self {
        self.opts.follow_root_links = yes;
        self
    }

    /// Follow symbolic links. By default, this is disabled.
    ///
    /// When `yes` is `true`, symbolic links are followed as if they were
    /// normal directories and files. If a symbolic link is broken or is
    /// involved in a loop, an error is yielded.
    ///
    /// When enabled, the yielded [`DirEntry`] values represent the target of
    /// the link while the path corresponds to the link. See the [`DirEntry`]
    /// type for more details.
    ///
    /// [`DirEntry`]: struct.DirEntry.html
    pub fn follow_links(mut self, yes: bool) -> Self {
        self.opts.follow_links = yes;
        self
    }
}

pub struct IntoIter {
    q: Vec<DirEntry>,
}

impl Iterator for IntoIter {
    type Item = Result<DirEntry>;

    fn next(&mut self) -> Option<Self::Item> {
        todo!()
    }
}

impl IntoIterator for WalkDir {
    type Item = Result<DirEntry>;

    type IntoIter = IntoIter;

    fn into_iter(self) -> Self::IntoIter {
        todo!()
    }
}

pub fn bfs<P: AsRef<Path>>(root: &P) -> Result<HashMap<String, Vec<String>>> {
    let rp = root.as_ref().to_path_buf();
    ensure!(rp.is_dir(), "root must be a directory");
    let rps = rp.clone().into_os_string().into_string().unwrap();

    let mut stack = vec![rp];

    let mut ret: HashMap<String, Vec<String>> = HashMap::new();
    ret.insert(rps.clone(), Vec::new());

    while let Some(parent) = stack.pop() {
        // Parent is always the next entry in the bfs tree
        let ps = parent.clone().into_os_string().into_string().unwrap();

        for entry in fs::read_dir(parent)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_dir() {
                stack.push(path);
            }

            let en: String = entry.file_name().into_string().unwrap();
            if ret.contains_key(&ps) {
                let v = &mut *ret.get_mut(&ps).unwrap();
                v.push(en)
            } else {
                ret.insert(ps.clone(), Vec::new());
            }
        }
    }

    Ok(ret)
}

package tasks

import (
	"os"
	"path"
	"strconv"

	"github.com/semaphoreui/semaphore/db"
	"github.com/semaphoreui/semaphore/db_lib"
	log "github.com/sirupsen/logrus"

	"github.com/semaphoreui/semaphore/util"
)

func (t *LocalJob) installInventory() (err error) {
	// Handle multiple inventories
	if len(t.Inventories) > 0 {
		for idx, inventory := range t.Inventories {
			// Install SSH and become keys only once (from first inventory)
			if idx == 0 {
				if inventory.SSHKeyID != nil {
					t.sshKeyInstallation, err = t.KeyInstaller.Install(inventory.SSHKey, db.AccessKeyRoleAnsibleUser, t.Logger)
					if err != nil {
						return
					}
				}

				if inventory.BecomeKeyID != nil {
					t.becomeKeyInstallation, err = t.KeyInstaller.Install(inventory.BecomeKey, db.AccessKeyRoleAnsibleBecomeUser, t.Logger)
					if err != nil {
						return
					}
				}
			}

			// Install each inventory file
			switch inventory.Type {
			case db.InventoryFile:
				err = t.cloneInventoryRepoForIndex(t.KeyInstaller, idx)
			case db.InventoryStatic, db.InventoryStaticYaml:
				err = t.installStaticInventoryForIndex(idx)
			}

			if err != nil {
				return
			}
		}
		return
	}

	// Fallback to single inventory for backward compatibility
	if t.Inventory.SSHKeyID != nil {
		t.sshKeyInstallation, err = t.KeyInstaller.Install(t.Inventory.SSHKey, db.AccessKeyRoleAnsibleUser, t.Logger)
		if err != nil {
			return
		}
	}

	if t.Inventory.BecomeKeyID != nil {
		t.becomeKeyInstallation, err = t.KeyInstaller.Install(t.Inventory.BecomeKey, db.AccessKeyRoleAnsibleBecomeUser, t.Logger)
		if err != nil {
			return
		}
	}

	switch t.Inventory.Type {
	case db.InventoryFile:
		err = t.cloneInventoryRepo(t.KeyInstaller)
	case db.InventoryStatic, db.InventoryStaticYaml:
		err = t.installStaticInventory()
	}

	return
}

func (t *LocalJob) tmpInventoryFilename(idx int) string {
	var inventory db.Inventory
	if len(t.Inventories) > idx {
		inventory = t.Inventories[idx]
	} else {
		inventory = t.Inventory
	}

	if inventory.Repository == nil {
		return "inventory_" + strconv.Itoa(inventory.ID)
	}
	return inventory.Repository.GetDirName(t.Template.ID) + "_inventory_" + strconv.Itoa(inventory.ID)
}

func (t *LocalJob) tmpInventoryFullPath(idx int) string {
	var inventory db.Inventory
	if len(t.Inventories) > idx {
		inventory = t.Inventories[idx]
	} else {
		inventory = t.Inventory
	}

	if inventory.Repository != nil && inventory.Repository.GetType() == db.RepositoryLocal {
		return inventory.Repository.GetGitURL(true)
	}
	pathname := path.Join(util.Config.GetProjectTmpDir(t.Template.ProjectID), t.tmpInventoryFilename(idx))
	if inventory.Type == db.InventoryStaticYaml {
		pathname += ".yml"
	}
	return pathname
}

func (t *LocalJob) cloneInventoryRepoForIndex(keyInstaller db_lib.AccessKeyInstaller, idx int) error {
	if idx >= len(t.Inventories) {
		return nil
	}

	inventory := t.Inventories[idx]
	if inventory.Repository == nil {
		return nil
	}

	if inventory.Repository.GetType() == db.RepositoryLocal {
		return nil
	}

	t.Log("cloning inventory repository " + strconv.Itoa(inventory.ID))

	repo := db_lib.GitRepository{
		Logger:     t.Logger,
		TmpDirName: t.tmpInventoryFilename(idx),
		Repository: *inventory.Repository,
		Client:     db_lib.CreateDefaultGitClient(keyInstaller),
	}

	// Try to pull the repo before trying to clone it
	if repo.CanBePulled() {
		err := repo.Pull()
		if err == nil {
			return nil
		}
	}

	err := os.RemoveAll(repo.GetFullPath())
	if err != nil {
		return err
	}

	return repo.Clone()
}

func (t *LocalJob) cloneInventoryRepo(keyInstaller db_lib.AccessKeyInstaller) error {
	if t.Inventory.Repository == nil {
		return nil
	}

	if t.Inventory.Repository.GetType() == db.RepositoryLocal {
		return nil
	}

	t.Log("cloning inventory repository")

	repo := db_lib.GitRepository{
		Logger:     t.Logger,
		TmpDirName: t.tmpInventoryFilename(0),
		Repository: *t.Inventory.Repository,
		Client:     db_lib.CreateDefaultGitClient(keyInstaller),
	}

	// Try to pull the repo before trying to clone it
	if repo.CanBePulled() {
		err := repo.Pull()
		if err == nil {
			return nil
		}
	}

	err := os.RemoveAll(repo.GetFullPath())
	if err != nil {
		return err
	}

	return repo.Clone()
}

func (t *LocalJob) installStaticInventoryForIndex(idx int) error {
	if idx >= len(t.Inventories) {
		return nil
	}

	inventory := t.Inventories[idx]
	t.Log("installing static inventory " + strconv.Itoa(inventory.ID))

	fullPath := t.tmpInventoryFullPath(idx)

	// create inventory file
	return os.WriteFile(fullPath, []byte(inventory.Inventory), 0664)
}

func (t *LocalJob) installStaticInventory() error {
	t.Log("installing static inventory")

	fullPath := t.tmpInventoryFullPath(0)

	// create inventory file
	return os.WriteFile(fullPath, []byte(t.Inventory.Inventory), 0664)
}

func (t *LocalJob) destroyInventoryFile() {
	// Handle multiple inventories
	if len(t.Inventories) > 0 {
		for idx, inventory := range t.Inventories {
			if !inventory.Type.IsStatic() {
				continue
			}

			fullPath := t.tmpInventoryFullPath(idx)
			if err := os.Remove(fullPath); err != nil {
				if os.IsNotExist(err) {
					continue
				}

				log.WithError(err).WithFields(log.Fields{
					"context": "task_running",
					"task_id": t.Task.ID,
				}).Warn("failed to remove inventory file")
			}
		}
		return
	}

	// Fallback for single inventory
	if !t.Inventory.Type.IsStatic() {
		return
	}

	fullPath := t.tmpInventoryFullPath(0)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return
		}

		log.WithError(err).WithFields(log.Fields{
			"context": "task_running",
			"task_id": t.Task.ID,
		}).Warn("failed to remove inventory file")
	}
}

func (t *LocalJob) destroyKeys() {
	err := t.sshKeyInstallation.Destroy()
	if err != nil {
		t.Log("Can't destroy inventory user key, error: " + err.Error())
	}

	err = t.becomeKeyInstallation.Destroy()
	if err != nil {
		t.Log("Can't destroy inventory become user key, error: " + err.Error())
	}

	for _, vault := range t.vaultFileInstallations {
		err = vault.Destroy()
		if err != nil {
			t.Log("Can't destroy inventory vault password file, error: " + err.Error())
		}
	}
}

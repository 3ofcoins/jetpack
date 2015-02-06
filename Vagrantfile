# -*- mode: ruby; -*-
Vagrant.configure("2") do |config|
  config.vm.guest = :freebsd
  config.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_freebsd-10.1_chef-provisionerless.box"
  config.vm.box = "opscode_freebsd-10.1_chef-provisionerless.box"
  # private network em1 for nfs mount
  config.vm.network "private_network", ip: "10.0.1.10"
  config.ssh.shell = "/bin/sh"
  # Use NFS as a shared folder
  config.vm.synced_folder ".", "/vagrant", :nfs => true, id: "vagrant-root"

  config.vm.provider :virtualbox do |vb|
    vb.customize ["modifyvm", :id, "--memory", "512"]
    vb.customize ["modifyvm", :id, "--cpus", "2"]
    vb.customize ["modifyvm", :id, "--hwvirtex", "on"]
    vb.customize ["modifyvm", :id, "--audio", "none"]
    vb.customize ["modifyvm", :id, "--nictype1", "virtio"]
    vb.customize ["modifyvm", :id, "--nictype2", "virtio"]
    #vb.gui = true
  end
  
  config.vm.provision "ansible" do |ansible|
    ansible.playbook = "contrib/ansible/vagrant-playbook.yml"
    ansible.sudo = true
    ansible.verbose = 'v'
  end


end

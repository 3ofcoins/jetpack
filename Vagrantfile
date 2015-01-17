# -*- mode: ruby; -*-
Vagrant.configure("2") do |config|
  config.vm.guest = :freebsd
  config.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_freebsd-10.1_chef-provisionerless.box"
  config.vm.box = "opscode_freebsd-10.1_chef-provisionerless.box"
  config.vm.network "private_network", ip: "10.0.1.10"
  #config.ssh.username = "vagrant"
  #config.ssh.password = "vagrant"
  config.ssh.shell = "/bin/sh"
  #config.ssh.insert_key = "true"
  # Use NFS as a shared folder
  config.vm.synced_folder ".", "/vagrant", :nfs => true, id: "vagrant-root"

  config.vm.provider :virtualbox do |vb|
    # vb.customize ["startvm", :id, "--type", "gui"]
    vb.customize ["modifyvm", :id, "--memory", "512"]
    vb.customize ["modifyvm", :id, "--cpus", "2"]
    vb.customize ["modifyvm", :id, "--hwvirtex", "on"]
    vb.customize ["modifyvm", :id, "--audio", "none"]
    vb.customize ["modifyvm", :id, "--nictype1", "virtio"]
    vb.customize ["modifyvm", :id, "--nictype2", "virtio"]
    vb.gui = true
  end

  config.vm.provision "shell" do |s|
    s.path = "provision/bootstrap_freebsd.sh"
  end
  
  config.vm.provision "ansible" do |ansible|
    ansible.playbook = "provision/ansible.yml"
    ansible.sudo = true
  end


end

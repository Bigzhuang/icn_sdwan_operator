# Sdewan operator

he sdewan operator is developed under kubebuilder framework

## Development

Project initialization(mostly, developers should not execute this step)
```
go mod init sdewan.akraino.org/sdewan
kubebuilder init --domain sdewan.akraino.org
```

To create new CRD and controller
```
kubebuilder create api --group batch --version  v1alpha1  --kind  Mwan3Policy
```

## Deployment

To Deploy dev env
1. Deploy icn
2. kubectl apply -f sample/*.yaml

-----
# Backup

## Install ICN

1. clone icn repo
2. cd icn and make the following change of Makefile
  ```
  jenkins@pod14-node2:/home/stack/cheng/icncheng$ git diff Makefile
  diff --git a/Makefile b/Makefile
  index d0e5b33..9ac687b 100644
  --- a/Makefile
  +++ b/Makefile
  @@ -160,9 +160,6 @@ verify_nestedk8s: prerequisite \
  
   bm_verify_nestedk8s: prerequisite \
           kud_bm_deploy_e2e \
  -        sdwan_verifier \
  -        kud_bm_reset \
  -       clean_bm_packages
  ```
3. vagrant up
4. login vagrant VM and execute the following commands
  ```
  sudo su
  cd /vagrant
  make bm_verify_nestedk8s
  ```

## Deployment

The API admission webhook depends on cert-manager so we need to install cert-manager first.

To install the CRD and the controller, we can follow this guide.
https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html

We have the image built and published at `integratedcloudnative/sdewan-controller:dev`. The openwrt
docker image we used for test is at `integratedcloudnative/openwrt:dev`. To use some other images,
we need to make configuration in `config/default/manager_image_patch.yaml`

The simple installation steps:
1. kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v0.11.0/cert-manager.yaml
2. kubectl apply -f sdewan-deploy.yaml

## Create Sdewan CNF docker image
1. update build/set_proxy file with required proxy for docker build
2. execute below commands to generate Sdewan CNF docker image which tagged with 'openwrt-1806-mwan3'
```
cd build
sudo bash build_image.sh
```


## References

- https://book.kubebuilder.io/
- https://openwrt.org/

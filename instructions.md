# Setting up the repositories and building the docs

# Clone the repos
1. Clone Shepherd: 
	git clone git@github.com:RoboConOxfordshire/shepherd.git

2. Init the sheep submodule:
	cd sheepsrc
	git submodule init
	git submodule update

3. 	1. Install NVM (node version manager)
	2. Using nvm install node version 8.1.0, `nvm install 8.1.0`
	3. `nvm use 8.1.0` 
		The nodejs version used now is v8.1.0, this version is from 2017, but seems to be the only version everything is compatible with.
	4. npm install
	5. npm run vuepress:build
	6. Check the errors, all I did to fix them is the following:
		sudo apt-get install imagemagick  -- install imagemagick used for the next command
		cd images
		find . -name "*.jpg" -exec mogrify -format png {} \;  -- for every jpg image create a png alternative
		cp upload-button.png upload.png
		cp run-button.png shepherd-run.png
		cp run-button.png run.png  


sudo pacman -Syu `cat ./install_hypr.txt`
#source ./install_omzsh.sh

# capture git required info
read -p "Enter your git username: " git_username
read -p "Enter your git email: " git_email
git config --global user.name $git_username
git config --global user.email $git_email


echo cloning Nvim
#nvim
if [ ! -d $HOME/.config/nvim ]; then
    git clone https://github.com/enslinn8/nvim.git $HOME/.config/nvim
fi


# tmux
echo copying tmux config
cp -r ./tmux ~/.config/  
if [ ! -L $HOME/scripts ]; then
    ln -s $HOME/Repos/.dotfiles/scripts $HOME/scripts
fi

# hyprland
echo copying hyprland config
cp $HOME/Repos/.dotfiles/hypr/hyprland.conf $HOME/.config/hypr/


curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.4/install.sh | bash;
source $HOME/.zshrc
bash nvm i --lts



# yay
sudo pacman -S --needed git base-devel
git clone https://aur.archlinux.org/yay.git
cd yay
makepkg -si
cd -

#BRAVE
yay -Sy brave-bin --noconfirm



#AWS
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "$HOME/Downloads/awscliv2.zip"
unzip $HOME/Downloads/awscliv2.zip -d $HOME/Downloads/
sudo $HOME/Downloads/aws/install

bash npm i -g aws-cdk

#angular 
npm i -g @angular/cli




